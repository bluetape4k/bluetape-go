package sqlcheckpoint_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/batch"
	"github.com/bluetape4k/bluetape-go/batch/sqlcheckpoint"
	"github.com/bluetape4k/bluetape-go/sqlkit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type exampleOrder struct {
	ID     int64
	Amount int64
}

type exampleCheckpoint struct {
	Offset int `json:"offset"`
}

type exampleCheckpointReader struct{}

func (*exampleCheckpointReader) Open(context.Context) error { return nil }
func (*exampleCheckpointReader) Read(context.Context) (exampleOrder, bool, error) {
	return exampleOrder{}, false, nil
}
func (*exampleCheckpointReader) Close(context.Context) error        { return nil }
func (*exampleCheckpointReader) Restore(context.Context, any) error { return nil }
func (*exampleCheckpointReader) Checkpoint(context.Context) (any, bool, error) {
	return exampleCheckpoint{Offset: 42}, true, nil
}

func ExampleNew() {
	ctx := context.Background()

	// A caller-owned deployer login may assume the non-login migration owner.
	// The runtime role is never a member of either deployment role.
	migrationDB, err := sql.Open("pgx", "postgres://deployer@primary/app")
	if err != nil {
		return
	}
	defer func() { _ = migrationDB.Close() }()

	migrationCtx, migrationCancel := context.WithTimeout(ctx, 30*time.Second)
	defer migrationCancel()
	// A controlled one-time deployer action must first run:
	// ALTER SCHEMA public OWNER TO sqlcheckpoint_migration_owner.
	// Every deployment then verifies that prerequisite; the role name alone
	// does not confer ownership.
	if err = verifyPublicSchemaOwnedByMigrationOwner(migrationCtx, migrationDB); err != nil {
		return
	}
	migrationTx, err := migrationDB.BeginTx(migrationCtx, nil)
	if err != nil {
		return
	}
	defer func() { _ = migrationTx.Rollback() }()
	for _, statement := range []string{
		"set local lock_timeout = '5s'",
		"set local statement_timeout = '10s'",
		"set local role sqlcheckpoint_migration_owner",
		"revoke create on schema public from public",
		sqlcheckpoint.SchemaSQL,
	} {
		if _, err = migrationTx.ExecContext(migrationCtx, statement); err != nil {
			return
		}
	}
	// This application-supplied check must validate the exact relation, owner,
	// constraints, RLS/triggers/rules, and schema/table/column ACL allowlist.
	// false requires proof that the runtime role has zero grants.
	if err = validateCheckpointCatalogAndACLs(migrationCtx, migrationTx, false); err != nil {
		return
	}
	for _, statement := range []string{
		"grant usage on schema public to app_runtime",
		"grant select, insert, update on public.bluetape_batch_checkpoints to app_runtime",
	} {
		if _, err = migrationTx.ExecContext(migrationCtx, statement); err != nil {
			return
		}
	}
	// true requires the exact effective grants, LOGIN NOINHERIT, no membership
	// or inheritance, and no grant option.
	if err = validateCheckpointCatalogAndACLs(migrationCtx, migrationTx, true); err != nil {
		return
	}
	if err = migrationTx.Commit(); err != nil {
		return
	}

	// The caller separately owns and closes the least-privilege runtime pool.
	runtimeDB, err := sql.Open("pgx", "postgres://runtime@primary/app")
	if err != nil {
		return
	}
	defer func() { _ = runtimeDB.Close() }()

	codec := sqlcheckpoint.Codec[exampleCheckpoint]{
		Encode: func(checkpoint exampleCheckpoint) ([]byte, error) {
			return json.Marshal(checkpoint)
		},
		Decode: func(payload []byte) (exampleCheckpoint, error) {
			var checkpoint exampleCheckpoint
			err := json.Unmarshal(payload, &checkpoint)
			return checkpoint, err
		},
	}
	atomicWriter, err := sqlcheckpoint.New(runtimeDB, sqlcheckpoint.Options{
		Namespace: "orders-v2",
	}, codec, func(ctx context.Context, tx sqlkit.Session, items []exampleOrder) error {
		for _, item := range items {
			if _, err := tx.ExecContext(
				ctx,
				"insert into processed_orders (id, amount) values ($1, $2)",
				item.ID,
				item.Amount,
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return
	}

	step, err := batch.NewAtomicStep(batch.AtomicStepOptions[exampleOrder, exampleOrder]{
		Name:          "persist-orders",
		ChunkSize:     100,
		Reader:        &exampleCheckpointReader{},
		Processor:     batch.IdentityProcessor[exampleOrder](),
		AtomicWriter:  atomicWriter,
		CheckpointKey: "tenant:blue",
	})
	if err != nil {
		return
	}
	_ = step
}

func ExampleWriter_commitUnknownRecovery() {
	ctx := context.Background()
	checkpointKey := "tenant:blue"
	runtimeDB, err := sql.Open("pgx", "postgres://runtime@primary/app")
	if err != nil {
		return
	}
	defer func() { _ = runtimeDB.Close() }()

	writer, err := sqlcheckpoint.New(
		runtimeDB,
		sqlcheckpoint.Options{Namespace: "orders-v2"},
		sqlcheckpoint.Codec[exampleCheckpoint]{
			Encode: func(value exampleCheckpoint) ([]byte, error) {
				return json.Marshal(value)
			},
			Decode: func(payload []byte) (exampleCheckpoint, error) {
				var value exampleCheckpoint
				err := json.Unmarshal(payload, &value)
				return value, err
			},
		},
		func(ctx context.Context, tx sqlkit.Session, items []exampleOrder) error {
			for _, item := range items {
				if _, err := tx.ExecContext(
					ctx,
					"insert into processed_orders (id, amount) values ($1, $2)",
					item.ID,
					item.Amount,
				); err != nil {
					return err
				}
			}
			return nil
		},
	)
	if err != nil {
		return
	}
	commitCtx, commitCancel := context.WithTimeout(ctx, 2*time.Second)
	_, commitErr := writer.Commit(
		commitCtx,
		checkpointKey,
		0,
		[]exampleOrder{{ID: 42, Amount: 1000}},
		exampleCheckpoint{Offset: 1},
	)
	commitCancel()
	var operationErr *sqlcheckpoint.OpError
	if errors.Is(commitErr, batch.ErrCommitUnknown) &&
		!errors.Is(commitErr, batch.ErrAtomicityUnknown) &&
		errors.As(commitErr, &operationErr) &&
		operationErr.Operation() == "commit" {
		quiesceCheckpointKey(checkpointKey)
		// Reconciliation gets a new bounded context only after the ambiguous
		// commit context has been cancelled.
		freshCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		checkpoint, exists, err := writer.Load(freshCtx, checkpointKey)
		if err != nil {
			return
		}
		// A fresh Load is authoritative only while same-key actors stay quiesced.
		_, _ = checkpoint, exists
	}
}

func ExampleAtomicityPanic() {
	checkpointKey := "tenant:blue"

	defer func() {
		recoverCheckpointPanic(
			checkpointKey,
			recover(),
			quiesceCheckpointKey,
			reconcileCheckpoint,
		)
	}()

	// Run the step inside this trusted top-level boundary. Do not log
	// AtomicityPanic.PanicValue; it may contain sensitive application data.
}

func verifyPublicSchemaOwnedByMigrationOwner(ctx context.Context, db *sql.DB) error {
	var owner string
	err := db.QueryRowContext(ctx, `select pg_catalog.pg_get_userbyid(n.nspowner)
from pg_catalog.pg_namespace n where n.nspname = 'public'`).Scan(&owner)
	if err != nil {
		return fmt.Errorf("verify public schema owner: %w", err)
	}
	if owner != "sqlcheckpoint_migration_owner" {
		return fmt.Errorf("public schema owner %q; want sqlcheckpoint_migration_owner", owner)
	}
	return nil
}

func validateCheckpointCatalogAndACLs(
	_ context.Context,
	_ sqlkit.Session,
	expectRuntimeGrants bool,
) error {
	return fmt.Errorf(
		"application must implement checkpoint catalog and ACL validation (expect runtime grants: %t)",
		expectRuntimeGrants,
	)
}

func recoverCheckpointPanic(
	checkpointKey string,
	recovered any,
	quiesce func(string),
	reconcile func(string),
) {
	if recovered == nil {
		return
	}
	recoveredErr, ok := recovered.(error)
	var atomicityPanic *sqlcheckpoint.AtomicityPanic
	if ok && errors.As(recoveredErr, &atomicityPanic) &&
		errors.Is(recoveredErr, batch.ErrAtomicityUnknown) {
		if quiesce == nil || reconcile == nil {
			panic("sqlcheckpoint example: application recovery hooks are required")
		}
		quiesce(checkpointKey)
		reconcile(checkpointKey)
		return
	}
	panic(recovered)
}

func quiesceCheckpointKey(string) {
	panic("application must stop intake and join every same-key run")
}

func reconcileCheckpoint(string) {
	panic("application must reconcile business state and checkpoint state")
}

func TestExampleRecoveryHooksFailClosed(t *testing.T) {
	for name, hook := range map[string]func(string){
		"quiesce":   quiesceCheckpointKey,
		"reconcile": reconcileCheckpoint,
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s hook silently returned without application behavior", name)
				}
			}()
			hook("tenant:blue")
		})
	}
}

func TestExampleRecoveryClassification(t *testing.T) {
	t.Run("typed atomicity panic invokes both application hooks", func(t *testing.T) {
		var calls []string
		recoverCheckpointPanic(
			"tenant:blue",
			&sqlcheckpoint.AtomicityPanic{},
			func(key string) { calls = append(calls, "quiesce:"+key) },
			func(key string) { calls = append(calls, "reconcile:"+key) },
		)
		want := []string{"quiesce:tenant:blue", "reconcile:tenant:blue"}
		if len(calls) != len(want) || calls[0] != want[0] || calls[1] != want[1] {
			t.Fatalf("recovery calls = %v, want %v", calls, want)
		}
	})

	t.Run("ordinary sentinel wrapping panic is rethrown", func(t *testing.T) {
		ordinary := errors.Join(errors.New("ordinary panic"), batch.ErrAtomicityUnknown)
		defer func() {
			recovered := recover()
			if recovered == nil ||
				reflect.ValueOf(recovered).Pointer() != reflect.ValueOf(ordinary).Pointer() {
				t.Fatalf("recovered = %#v, want original %#v", recovered, ordinary)
			}
		}()
		recoverCheckpointPanic(
			"tenant:blue",
			ordinary,
			func(string) { t.Fatal("ordinary panic invoked quiesce") },
			func(string) { t.Fatal("ordinary panic invoked reconcile") },
		)
	})
}
