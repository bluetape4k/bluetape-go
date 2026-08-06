package sqlkit_test

import (
	"context"
	"database/sql"
	"database/sql/driver"

	"github.com/bluetape4k/bluetape-go/encrypt"
	"github.com/bluetape4k/bluetape-go/sqlkit"
)

func ExampleJSONColumn_databaseSQL() {
	query := func(ctx context.Context, db *sql.DB, id int64) (sqlkit.JSONColumn[jsonProfile], error) {
		var profile sqlkit.JSONColumn[jsonProfile]
		err := db.QueryRowContext(ctx, "select profile from accounts where id = $1", id).Scan(&profile)
		return profile, err
	}
	save := func(ctx context.Context, db *sql.DB, id int64, profile jsonProfile) error {
		value := sqlkit.JSONColumn[jsonProfile]{Data: profile, Valid: true}
		_, err := db.ExecContext(ctx, "update accounts set profile = $1 where id = $2", value, id)
		return err
	}

	_, _ = query, save
}

func ExampleEncryptedBytesColumn_sqlkit() {
	load := func(ctx context.Context, db sqlkit.Queryer, encryptor encrypt.Encryptor, id int64) ([]byte, error) {
		return sqlkit.QueryOne(ctx, db, "select payload from secrets where id = $1", func(rows *sql.Rows) ([]byte, error) {
			column := sqlkit.NewEncryptedBytesColumn(encryptor, []byte("table=secrets:column=payload"))
			if err := rows.Scan(&column); err != nil {
				return nil, err
			}
			return append([]byte(nil), column.Data...), nil
		}, id)
	}

	_ = load
}

type generatedUpdateSecretParams struct {
	ID      int64
	Payload driver.Valuer
}

type generatedSecretQueries interface {
	UpdateSecret(context.Context, generatedUpdateSecretParams) error
}

func ExampleEncryptedStringColumn_generatedQuery() {
	save := func(ctx context.Context, queries generatedSecretQueries, encryptor encrypt.Encryptor, id int64, text string) error {
		column := sqlkit.NewEncryptedStringColumn(encryptor, []byte("table=secrets:column=note"))
		column.Data = text
		column.Valid = true
		return queries.UpdateSecret(ctx, generatedUpdateSecretParams{ID: id, Payload: column})
	}

	_ = save
}
