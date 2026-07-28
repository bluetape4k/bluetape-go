package sqloutbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/sqlkit"
)

const (
	defaultTable         = "audit_outbox"
	defaultMaxEntryBytes = 1 << 20
	defaultMaxErrorBytes = 2048
	defaultLeaseDuration = 30 * time.Second
)

var (
	// ErrInvalidArgument sqloutbox API 인자가 유효하지 않을 때 반환된다.
	ErrInvalidArgument = errors.New("invalid sqloutbox argument")
	// ErrInvalidRecord 저장된 outbox record 상태가 유효하지 않을 때 반환된다.
	ErrInvalidRecord = errors.New("invalid sqloutbox record")
	// ErrRecordNotFound 전이 대상 outbox record가 더 이상 기대 상태에 없을 때 반환된다.
	ErrRecordNotFound = errors.New("sqloutbox record not found")
)

// Status string 공개 타입이며 SQL outbox store, relay, transaction, idempotent delivery 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Status string

const (
	// StatusPending available_at 이후 relay가 record를 점유할 수 있는 상태다.
	StatusPending Status = "pending"
	// StatusClaimed relay가 publish를 위해 record lease를 점유한 상태다.
	StatusClaimed Status = "claimed"
	// StatusPublished publisher 전송이 성공한 상태다.
	StatusPublished Status = "published"
	// StatusDeadLetter 재시도 한도를 소진해 더 이상 자동 전송하지 않는 상태다.
	StatusDeadLetter Status = "dead_letter"
)

// RecordID int64 공개 타입이며 SQL outbox store, relay, transaction, idempotent delivery 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type RecordID int64

// Options struct 공개 타입이며 SQL outbox store, relay, transaction, idempotent delivery 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Options struct {
	// Table은 PostgreSQL table 이름이다. "audit.audit_outbox" 같은 schema-qualified 이름을 허용한다.
	Table string
	// Now write timestamp에 사용할 wall-clock 시간을 공급한다. nil이면 time.Now를 사용한다.
	Now func() time.Time
	// MaxEntryBytes database에서 decode할 Entry JSON byte 한도를 지정한다.
	MaxEntryBytes int
	// MaxErrorBytes 저장할 publish failure 문자열의 byte 한도를 지정한다.
	MaxErrorBytes int
}

// Store struct 공개 타입이며 SQL outbox store, relay, transaction, idempotent delivery 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Store struct {
	table         string
	quotedTable   string
	claimIndex    string
	now           func() time.Time
	maxEntryBytes int
	maxErrorBytes int
}

// Record struct 공개 타입이며 SQL outbox store, relay, transaction, idempotent delivery 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Record struct {
	ID             RecordID
	Status         Status
	Aggregate      audit.AggregateID
	Revision       audit.Revision
	EventID        audit.EventID
	IdempotencyKey string
	EventType      audit.EventType
	OccurredAt     time.Time
	RecordedAt     time.Time
	SchemaVersion  int
	Attempts       int
	Entry          audit.Entry
}

// ClaimOptions struct 공개 타입이며 SQL outbox store, relay, transaction, idempotent delivery 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type ClaimOptions struct {
	Limit         int
	Now           time.Time
	LeaseDuration time.Duration
}

// Failure struct 공개 타입이며 SQL outbox store, relay, transaction, idempotent delivery 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성/transaction 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Failure struct {
	ID          RecordID
	Attempt     int
	Err         error
	RetryAt     time.Time
	MaxAttempts int
	Now         time.Time
}

// NewStore NewStore 공개 API의 동작을 수행하며 SQL outbox store, relay, transaction, idempotent delivery 계약을 보존한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func NewStore(options Options) (*Store, error) {
	table := strings.TrimSpace(options.Table)
	if table == "" {
		table = defaultTable
	}
	quotedTable, err := quoteQualifiedIdentifier(table)
	if err != nil {
		return nil, err
	}

	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	maxEntryBytes := options.MaxEntryBytes
	if maxEntryBytes == 0 {
		maxEntryBytes = defaultMaxEntryBytes
	}
	if maxEntryBytes < 0 {
		return nil, fmt.Errorf("%w: max entry bytes must not be negative", ErrInvalidArgument)
	}
	maxErrorBytes := options.MaxErrorBytes
	if maxErrorBytes == 0 {
		maxErrorBytes = defaultMaxErrorBytes
	}
	if maxErrorBytes < 0 {
		return nil, fmt.Errorf("%w: max error bytes must not be negative", ErrInvalidArgument)
	}

	return &Store{
		table:         table,
		quotedTable:   quotedTable,
		claimIndex:    quoteIdentifier(indexName(table, "claim_idx")),
		now:           now,
		maxEntryBytes: maxEntryBytes,
		maxErrorBytes: maxErrorBytes,
	}, nil
}

// CreateSchema CreateSchema 공개 API의 동작을 수행하며 SQL outbox store, relay, transaction, idempotent delivery 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - db: SQL transaction 또는 outbox 저장소 backend다. commit/rollback 소유권은 호출자와 store 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (s *Store) CreateSchema(ctx context.Context, db sqlkit.Execer) error {
	if err := requireStoreAndExecer(s, db); err != nil {
		return err
	}

	createTable := fmt.Sprintf(`
create table if not exists %s (
	id bigserial primary key,
	status text not null,
	aggregate_type text not null,
	aggregate_id text not null,
	revision bigint not null,
	event_id text not null unique,
	idempotency_key text not null unique,
	event_type text not null,
	occurred_at timestamptz not null,
	recorded_at timestamptz not null,
	schema_version integer not null,
	entry_json jsonb not null,
	attempts integer not null default 0,
	available_at timestamptz not null,
	claimed_at timestamptz,
	published_at timestamptz,
	dead_lettered_at timestamptz,
	last_error text not null default '',
	created_at timestamptz not null,
	updated_at timestamptz not null
)`, s.quotedTable)
	if _, err := db.ExecContext(ctx, createTable); err != nil {
		return err
	}

	createIndex := fmt.Sprintf(
		`create index if not exists %s on %s (status, available_at, aggregate_type, aggregate_id, revision, id)`,
		s.claimIndex,
		s.quotedTable,
	)
	_, err := db.ExecContext(ctx, createIndex)
	return err
}

// Enqueue Enqueue 공개 API의 동작을 수행하며 SQL outbox store, relay, transaction, idempotent delivery 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - db: SQL transaction 또는 outbox 저장소 backend다. commit/rollback 소유권은 호출자와 store 계약을 따른다.
//   - entries: Enqueue에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (s *Store) Enqueue(ctx context.Context, db sqlkit.Execer, entries ...audit.Entry) error {
	if err := requireStoreAndExecer(s, db); err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}

	query := fmt.Sprintf(`
insert into %s (
	status, aggregate_type, aggregate_id, revision, event_id, idempotency_key,
	event_type, occurred_at, recorded_at, schema_version, entry_json, attempts,
	available_at, created_at, updated_at
) values (
	$1, $2, $3, $4, $5, $6,
	$7, $8, $9, $10, $11, 0,
	$12, $13, $13
)`, s.quotedTable)

	now := s.now()
	for _, entry := range entries {
		if err := entry.Validate(); err != nil {
			return err
		}
		encoded, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if len(encoded) > s.maxEntryBytes {
			return fmt.Errorf("%w: entry_json exceeds %d bytes", ErrInvalidRecord, s.maxEntryBytes)
		}
		if _, err := db.ExecContext(ctx, query,
			string(StatusPending),
			entry.Aggregate.Type,
			entry.Aggregate.ID,
			uint64(entry.Revision),
			string(entry.Event.EventID),
			entry.Event.IdempotencyKey,
			string(entry.Event.EventType),
			entry.Event.OccurredAt,
			entry.Event.RecordedAt,
			entry.SchemaVersion,
			encoded,
			entry.Event.RecordedAt,
			now,
		); err != nil {
			return err
		}
	}
	return nil
}

// Claim Claim 공개 API의 동작을 수행하며 SQL outbox store, relay, transaction, idempotent delivery 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - db: SQL transaction 또는 outbox 저장소 backend다. commit/rollback 소유권은 호출자와 store 계약을 따른다.
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (s *Store) Claim(ctx context.Context, db sqlkit.Session, options ClaimOptions) ([]Record, error) {
	if err := requireStoreAndSession(s, db); err != nil {
		return nil, err
	}
	if options.Limit <= 0 {
		return nil, fmt.Errorf("%w: claim limit must be positive", ErrInvalidArgument)
	}
	leaseDuration := options.LeaseDuration
	if leaseDuration == 0 {
		leaseDuration = defaultLeaseDuration
	}
	if leaseDuration < 0 {
		return nil, fmt.Errorf("%w: lease duration must not be negative", ErrInvalidArgument)
	}
	now := options.Now
	if now.IsZero() {
		now = s.now()
	}
	leaseUntil := now.Add(leaseDuration)

	query := fmt.Sprintf(`
with candidates as (
	select candidate.id
	from %s candidate
	where candidate.status in ($1, $2)
	  and candidate.available_at <= $3
	  and not exists (
		select 1
		from %s earlier
		where earlier.aggregate_type = candidate.aggregate_type
		  and earlier.aggregate_id = candidate.aggregate_id
		  and earlier.revision < candidate.revision
		  and earlier.status in ($1, $2)
	  )
	order by candidate.aggregate_type, candidate.aggregate_id, candidate.revision, candidate.id
	limit $4
	for update skip locked
)
update %s outbox
set status = $2,
	attempts = outbox.attempts + 1,
	claimed_at = $3,
	available_at = $5,
	updated_at = $3,
	last_error = ''
from candidates
where outbox.id = candidates.id
returning outbox.id, outbox.status, outbox.aggregate_type, outbox.aggregate_id,
	outbox.revision, outbox.event_id, outbox.idempotency_key, outbox.event_type,
	outbox.occurred_at, outbox.recorded_at, outbox.schema_version, outbox.entry_json,
	outbox.attempts`,
		s.quotedTable,
		s.quotedTable,
		s.quotedTable,
	)

	rows, err := db.QueryContext(ctx, query,
		string(StatusPending),
		string(StatusClaimed),
		now,
		options.Limit,
		leaseUntil,
	)
	if err != nil {
		return nil, err
	}
	records, scanErr := s.scanRecords(rows)
	closeErr := rows.Close()
	if scanErr != nil {
		return nil, scanErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return records, nil
}

// MarkPublished MarkPublished 공개 API의 동작을 수행하며 SQL outbox store, relay, transaction, idempotent delivery 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - db: SQL transaction 또는 outbox 저장소 backend다. commit/rollback 소유권은 호출자와 store 계약을 따른다.
//   - records: MarkPublished에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (s *Store) MarkPublished(ctx context.Context, db sqlkit.Execer, records ...Record) error {
	if err := requireStoreAndExecer(s, db); err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	query := fmt.Sprintf(
		`update %s set status = $1, published_at = $2, updated_at = $2 where status = $3 and id = $4 and attempts = $5`,
		s.quotedTable,
	)
	now := s.now()
	for _, record := range records {
		if record.ID <= 0 {
			return fmt.Errorf("%w: record id must be positive", ErrInvalidArgument)
		}
		if record.Attempts <= 0 {
			return fmt.Errorf("%w: claim attempt must be positive", ErrInvalidArgument)
		}
		result, err := db.ExecContext(ctx, query,
			string(StatusPublished),
			now,
			string(StatusClaimed),
			int64(record.ID),
			record.Attempts,
		)
		if err != nil {
			return err
		}
		if err := requireRows(result, 1); err != nil {
			return err
		}
	}
	return nil
}

// MarkFailed MarkFailed 공개 API의 동작을 수행하며 SQL outbox store, relay, transaction, idempotent delivery 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - db: SQL transaction 또는 outbox 저장소 backend다. commit/rollback 소유권은 호출자와 store 계약을 따른다.
//   - failure: MarkFailed에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, transaction 실패, repository/outbox 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (s *Store) MarkFailed(ctx context.Context, db sqlkit.Execer, failure Failure) error {
	if err := requireStoreAndExecer(s, db); err != nil {
		return err
	}
	if failure.ID <= 0 {
		return fmt.Errorf("%w: record id must be positive", ErrInvalidArgument)
	}
	if failure.Attempt <= 0 {
		return fmt.Errorf("%w: claim attempt must be positive", ErrInvalidArgument)
	}
	if failure.MaxAttempts <= 0 {
		return fmt.Errorf("%w: max attempts must be positive", ErrInvalidArgument)
	}
	now := failure.Now
	if now.IsZero() {
		now = s.now()
	}
	retryAt := failure.RetryAt
	if retryAt.IsZero() {
		retryAt = now
	}
	lastError := s.sanitizeFailure(failure.Err)

	query := fmt.Sprintf(`
update %s
set status = case when attempts >= $2 then $3 else $4 end,
	available_at = case when attempts >= $2 then available_at else $5::timestamptz end,
	dead_lettered_at = case when attempts >= $2 then $6::timestamptz else null::timestamptz end,
	last_error = $7,
	updated_at = $6
where id = $1 and status = $8 and attempts = $9`, s.quotedTable)
	result, err := db.ExecContext(ctx, query,
		int64(failure.ID),
		failure.MaxAttempts,
		string(StatusDeadLetter),
		string(StatusPending),
		retryAt,
		now,
		lastError,
		string(StatusClaimed),
		failure.Attempt,
	)
	if err != nil {
		return err
	}
	return requireRows(result, 1)
}

func (s *Store) scanRecords(rows *sql.Rows) ([]Record, error) {
	records := make([]Record, 0)
	for rows.Next() {
		var record Record
		var status string
		var aggregateType string
		var aggregateID string
		var revision int64
		var eventID string
		var eventType string
		var entryJSON []byte
		if err := rows.Scan(
			&record.ID,
			&status,
			&aggregateType,
			&aggregateID,
			&revision,
			&eventID,
			&record.IdempotencyKey,
			&eventType,
			&record.OccurredAt,
			&record.RecordedAt,
			&record.SchemaVersion,
			&entryJSON,
			&record.Attempts,
		); err != nil {
			return nil, err
		}
		if revision <= 0 {
			return nil, fmt.Errorf("%w: revision must be positive", ErrInvalidRecord)
		}
		aggregate, err := audit.NewAggregateID(aggregateType, aggregateID)
		if err != nil {
			return nil, err
		}
		entry, err := s.decodeEntry(entryJSON)
		if err != nil {
			return nil, err
		}
		record.Status = Status(status)
		record.Aggregate = aggregate
		record.Revision = audit.Revision(revision)
		record.EventID = audit.EventID(eventID)
		record.EventType = audit.EventType(eventType)
		record.Entry = entry
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) decodeEntry(data []byte) (audit.Entry, error) {
	if len(data) == 0 {
		return audit.Entry{}, fmt.Errorf("%w: empty entry_json", ErrInvalidRecord)
	}
	if len(data) > s.maxEntryBytes {
		return audit.Entry{}, fmt.Errorf("%w: entry_json exceeds %d bytes", ErrInvalidRecord, s.maxEntryBytes)
	}
	return audit.DecodeEntryJSON(data)
}

func (s *Store) sanitizeFailure(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > s.maxErrorBytes {
		message = message[:s.maxErrorBytes]
	}
	return message
}

func requireStoreAndExecer(store *Store, db sqlkit.Execer) error {
	if store == nil {
		return fmt.Errorf("%w: store must not be nil", ErrInvalidArgument)
	}
	if db == nil {
		return fmt.Errorf("%w: session must not be nil", ErrInvalidArgument)
	}
	return nil
}

func requireStoreAndSession(store *Store, db sqlkit.Session) error {
	if store == nil {
		return fmt.Errorf("%w: store must not be nil", ErrInvalidArgument)
	}
	if db == nil {
		return fmt.Errorf("%w: session must not be nil", ErrInvalidArgument)
	}
	return nil
}

func requireRows(result sql.Result, want int) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != int64(want) {
		return fmt.Errorf("%w: updated %d of %d rows", ErrRecordNotFound, rows, want)
	}
	return nil
}

func quoteQualifiedIdentifier(name string) (string, error) {
	parts := strings.Split(name, ".")
	for _, part := range parts {
		if !validIdentifier(part) {
			return "", fmt.Errorf("%w: invalid table identifier %q", ErrInvalidArgument, name)
		}
	}
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = quoteIdentifier(part)
	}
	return strings.Join(quoted, "."), nil
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func indexName(table string, suffix string) string {
	normalized := strings.NewReplacer(".", "_").Replace(table)
	return normalized + "_" + suffix
}
