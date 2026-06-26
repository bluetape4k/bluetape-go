package sqlkit_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

type account struct {
	ID   int
	Name string
}

type accountRepository struct{}

func (accountRepository) Create(ctx context.Context, db sqlkit.Execer, value account) error {
	stmt, err := sqlkit.InsertInto("sqlkit_repo_accounts").
		Columns("id", "name").
		Values(value.ID, value.Name).
		Build()
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

func (accountRepository) Find(ctx context.Context, db sqlkit.Queryer, id int) (account, bool, error) {
	stmt, err := sqlkit.SelectFrom("sqlkit_repo_accounts").
		Columns("id", "name").
		Where("id = ?", id).
		Build()
	if err != nil {
		return account{}, false, err
	}
	return sqlkit.QueryOptional(ctx, db, stmt.SQL, scanAccount, stmt.Args...)
}

func (accountRepository) Rename(ctx context.Context, db sqlkit.Execer, id int, name string) error {
	stmt, err := sqlkit.Update("sqlkit_repo_accounts").
		Set("name", name).
		Where("id = ?", id).
		Build()
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

func (accountRepository) Delete(ctx context.Context, db sqlkit.Execer, id int) error {
	stmt, err := sqlkit.DeleteFrom("sqlkit_repo_accounts").
		Where("id = ?", id).
		Build()
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

func (accountRepository) AddEvent(ctx context.Context, db sqlkit.Execer, accountID int, kind string) error {
	stmt, err := sqlkit.InsertInto("sqlkit_repo_account_events").
		Columns("account_id", "kind").
		Values(accountID, kind).
		Build()
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

func (accountRepository) FindByEventKind(ctx context.Context, db sqlkit.Queryer, kind string) ([]account, error) {
	stmt, err := sqlkit.SelectFrom("sqlkit_repo_accounts").
		Columns("id", "name").
		Where("exists (select 1 from sqlkit_repo_account_events where sqlkit_repo_account_events.account_id = sqlkit_repo_accounts.id and sqlkit_repo_account_events.kind = ?)", kind).
		OrderBy("id").
		Build()
	if err != nil {
		return nil, err
	}
	return sqlkit.QueryAll(ctx, db, stmt.SQL, scanAccount, stmt.Args...)
}

func TestRepositoryPrototypePostgresCRUDRollbackAndRelationalQuery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	createRepositorySchema(ctx, t, db)
	repo := accountRepository{}

	if err := repo.Create(ctx, db, account{ID: 1, Name: "Ada"}); err != nil {
		t.Fatalf("create account: %v", err)
	}
	got, ok, err := repo.Find(ctx, db, 1)
	if err != nil {
		t.Fatalf("find account: %v", err)
	}
	if !ok {
		t.Fatal("created account was not found")
	}
	if got != (account{ID: 1, Name: "Ada"}) {
		t.Fatalf("account = %#v, want Ada account", got)
	}

	if err := repo.Rename(ctx, db, 1, "Grace"); err != nil {
		t.Fatalf("rename account: %v", err)
	}
	got, ok, err = repo.Find(ctx, db, 1)
	if err != nil {
		t.Fatalf("find renamed account: %v", err)
	}
	if !ok || got.Name != "Grace" {
		t.Fatalf("renamed account = %#v, ok=%v; want Grace", got, ok)
	}

	if err := repo.AddEvent(ctx, db, 1, "created"); err != nil {
		t.Fatalf("add event: %v", err)
	}
	withCreatedEvent, err := repo.FindByEventKind(ctx, db, "created")
	if err != nil {
		t.Fatalf("find by event kind: %v", err)
	}
	if len(withCreatedEvent) != 1 || withCreatedEvent[0].ID != 1 {
		t.Fatalf("accounts with created event = %#v, want account 1", withCreatedEvent)
	}

	rollbackErr := errors.New("rollback account")
	err = sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
		if err := repo.Create(ctx, tx, account{ID: 2, Name: "Rolled Back"}); err != nil {
			return err
		}
		if err := repo.AddEvent(ctx, tx, 2, "created"); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("rollback error = %v, want rollbackErr", err)
	}
	if _, ok, err := repo.Find(ctx, db, 2); err != nil {
		t.Fatalf("find rolled-back account: %v", err)
	} else if ok {
		t.Fatal("rolled-back account is visible")
	}

	if err := repo.Delete(ctx, db, 1); err != nil {
		t.Fatalf("delete account: %v", err)
	}
	if _, ok, err := repo.Find(ctx, db, 1); err != nil {
		t.Fatalf("find deleted account: %v", err)
	} else if ok {
		t.Fatal("deleted account is visible")
	}
}

func createRepositorySchema(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	statements := []string{
		`create table sqlkit_repo_accounts (id integer primary key, name text not null)`,
		`create table sqlkit_repo_account_events (
			id bigserial primary key,
			account_id integer not null references sqlkit_repo_accounts(id) on delete cascade,
			kind text not null
		)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("create repository schema: %v", err)
		}
	}
}

func scanAccount(rows *sql.Rows) (account, error) {
	var value account
	if err := rows.Scan(&value.ID, &value.Name); err != nil {
		return account{}, err
	}
	return value, nil
}
