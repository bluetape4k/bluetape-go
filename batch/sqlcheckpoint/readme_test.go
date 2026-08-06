package sqlcheckpoint

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var markdownRelativeLink = regexp.MustCompile(`\[[^]]*\]\(([^)]+)\)`)

func TestReadmeContract(t *testing.T) {
	readmes := map[string][]string{
		"README.md": {
			"NewAtomicStep",
			"SchemaSQL",
			"ErrCommitUnknown",
			"ErrAtomicityUnknown",
			"AtomicityPanic",
			"PanicValue",
			"SAVEPOINT",
			"authenticated codec",
			"KeyID",
			"authorization identifier",
			"metric label",
			"same `(namespace, key)`",
			"migration role",
			"runtime role",
			"quiesceCheckpointKey",
			"reconcileCheckpoint",
			"deployer login",
			"non-login migration owner",
			"LOGIN NOINHERIT",
			"no role membership",
			"NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS",
			"WITH INHERIT FALSE, SET TRUE, ADMIN FALSE",
			"inbound and outbound role membership",
			"not an authorization boundary",
			"Read Committed",
			"ambient isolation",
			"safe for concurrent use",
			"report.Err",
			"sqlcheckpoint.OperationCommit",
			"preflight before runtime grants",
			"*sqlcheckpoint.OpError",
			"Operation() == sqlcheckpoint.OperationCommit",
			"errors.As",
			"public schema ownership prerequisite",
			"ALTER SCHEMA public OWNER TO sqlcheckpoint_migration_owner",
			"pre-grant catalog/ACL validation",
			"zero runtime grants",
			"post-grant effective privilege validation",
			"zero role membership",
			"zero inheritance",
			"no grant option",
			"go test -count=1 ./batch ./batch/sqlcheckpoint -run 'Example|README|Readme'",
		},
		"README.ko.md": {
			"NewAtomicStep",
			"SchemaSQL",
			"ErrCommitUnknown",
			"ErrAtomicityUnknown",
			"AtomicityPanic",
			"PanicValue",
			"SAVEPOINT",
			"authenticated codec",
			"KeyID",
			"authorization 식별자",
			"metric label",
			"같은 `(namespace, key)`",
			"migration role",
			"runtime role",
			"quiesceCheckpointKey",
			"reconcileCheckpoint",
			"deployer login",
			"non-login migration owner",
			"LOGIN NOINHERIT",
			"role membership 없음",
			"NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS",
			"WITH INHERIT FALSE, SET TRUE, ADMIN FALSE",
			"inbound/outbound `role membership 없음`",
			"authorization 경계가 아닙니다",
			"Read Committed",
			"ambient isolation",
			"concurrent-safe",
			"report.Err",
			"sqlcheckpoint.OperationCommit",
			"runtime grant 전 preflight",
			"*sqlcheckpoint.OpError",
			"Operation() == sqlcheckpoint.OperationCommit",
			"errors.As",
			"public schema ownership prerequisite",
			"ALTER SCHEMA public OWNER TO sqlcheckpoint_migration_owner",
			"pre-grant catalog/ACL validation",
			"zero runtime grants",
			"post-grant effective privilege validation",
			"zero role membership",
			"zero inheritance",
			"no grant option",
			"go test -count=1 ./batch ./batch/sqlcheckpoint -run 'Example|README|Readme'",
		},
	}

	fenceCounts := make(map[string]int, len(readmes))
	for name, markers := range readmes {
		name, markers := name, markers
		t.Run(name, func(t *testing.T) {
			body := readContractFile(t, name)
			assertContractMarkers(t, body, markers)
			assertOrderedContractMarkers(t, body, []string{
				"verifyPublicSchemaOwnedByMigrationOwner",
				`"set local lock_timeout = '5s'"`,
				`"set local statement_timeout = '10s'"`,
				`"set local role sqlcheckpoint_migration_owner"`,
				`"revoke create on schema public from public"`,
				"sqlcheckpoint.SchemaSQL",
				"validateCheckpointCatalogAndACLs(migrationCtx, migrationTx, false)",
				`"grant usage on schema public to app_runtime"`,
				"validateCheckpointCatalogAndACLs(migrationCtx, migrationTx, true)",
				"migrationTx.Commit()",
			})
			assertContractMarkers(t, body, []string{
				"postgres-batch-checkpoint-atomic-sequence.png",
				"RetryPolicy",
				"SkipPolicy",
				"processor",
				"AtomicCheckpointWriter.Commit",
				"callback",
				"CAS",
				"unknown",
			})
			fenceCounts[name] = strings.Count(body, "```")
			if fenceCounts[name]%2 != 0 {
				t.Errorf("unbalanced fenced-code-block markers: %d", fenceCounts[name])
			}
			assertRelativeLinks(t, name, body)
		})
	}

	if fenceCounts["README.md"] != fenceCounts["README.ko.md"] {
		t.Errorf(
			"locale fenced-code-block marker counts differ: English=%d Korean=%d",
			fenceCounts["README.md"],
			fenceCounts["README.ko.md"],
		)
	}
}

func TestReadmeHeadingParity(t *testing.T) {
	english := readContractFile(t, "README.md")
	korean := readContractFile(t, "README.ko.md")

	englishHeadings := markdownH2s(english)
	koreanHeadings := markdownH2s(korean)
	if len(englishHeadings) != len(koreanHeadings) {
		t.Fatalf("locale H2 counts differ: English=%d Korean=%d", len(englishHeadings), len(koreanHeadings))
	}
	wantEnglish := []string{
		"Architecture And Selection",
		"Install",
		"Pool Ownership And Schema Bootstrap",
		"JSON Codec And Atomic Step Construction",
		"Callback Transaction Contract",
		"Progress, Revisions, And Conflicts",
		"Payload, Codec, And Encryption",
		"Policy Boundary",
		"Errors And Recovery",
		"Rollout, Rollback, And Retention",
		"Operations And Supported Topology",
		"Validation",
	}
	wantKorean := []string{
		"구조와 선택 기준",
		"설치",
		"Pool 소유권과 schema bootstrap",
		"JSON codec과 atomic step 구성",
		"Callback transaction 계약",
		"Progress, revision, conflict",
		"Payload, codec, encryption",
		"Policy 경계",
		"Error와 recovery",
		"Rollout, rollback, retention",
		"운영과 지원 topology",
		"검증",
	}
	assertStringSlicesEqual(t, "English H2 structure", englishHeadings, wantEnglish)
	assertStringSlicesEqual(t, "Korean H2 structure", koreanHeadings, wantKorean)
}

func TestExampleMigrationAndRecoveryOrderContract(t *testing.T) {
	body := readContractFile(t, "example_test.go")
	commitRecovery := body[strings.Index(body, "func ExampleWriter_commitUnknownRecovery"):]
	assertOrderedContractMarkers(t, body, []string{
		"verifyPublicSchemaOwnedByMigrationOwner(migrationCtx, migrationDB)",
		"migrationDB.BeginTx(migrationCtx, nil)",
		`"set local role sqlcheckpoint_migration_owner"`,
		`"revoke create on schema public from public"`,
		"sqlcheckpoint.SchemaSQL",
		"validateCheckpointCatalogAndACLs(migrationCtx, migrationTx, false)",
		`"grant usage on schema public to app_runtime"`,
		"validateCheckpointCatalogAndACLs(migrationCtx, migrationTx, true)",
		"migrationTx.Commit()",
	})
	assertOrderedContractMarkers(t, body, []string{
		"step.Run(runCtx)",
		"runErr := report.Err",
		"errors.Is(runErr, batch.ErrCommitUnknown)",
		"operationErr.Operation() == sqlcheckpoint.OperationCommit",
		"atomicWriter.Load(freshCtx, checkpointKey)",
	})
	assertOrderedContractMarkers(t, commitRecovery, []string{
		"commitCtx, commitCancel := context.WithTimeout",
		"writer.Commit(",
		"commitCancel()",
		"freshCtx, cancel := context.WithTimeout",
		"writer.Load(freshCtx, checkpointKey)",
	})
}

func TestSequenceDiagramOwnershipContract(t *testing.T) {
	body := readContractFile(t, "../../docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.svg")
	assertContractMarkers(t, body, []string{
		`d="M180 300 L900 300"`,
		`d="M900 360 L1280 360"`,
		`d="M1280 420 L900 420"`,
		`d="M180 480 L540 480"`,
		`d="M180 1380 L900 1380"`,
		"Restore(checkpoint value)",
		"caller: quiesce; Atomic Writer.Load",
	})
	if strings.Contains(body, `<rect class="activation" x="533" y="1358"`) {
		t.Error("commit-unknown recovery must not activate CheckpointReader")
	}
}

func TestAtomicPolicyDocumentationContract(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"README.md", "README.ko.md", "../doc.go"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			body := readContractFile(t, name)
			assertContractMarkers(t, body, []string{
				"RetryPolicy",
				"SkipPolicy",
				"processor failures",
				"AtomicCheckpointWriter.Commit",
				"callback",
				"CAS",
				"unknown-outcome",
			})
		})
	}
}

func TestLegacyBatchDocumentationContract(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"../README.md", "../README.ko.md"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			body := readContractFile(t, name)
			assertContractMarkers(t, body, []string{
				"Writer + CheckpointStore",
				"durable",
				"not atomic with business writes",
			})
		})
	}
}

func readContractFile(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(body)
}

func assertContractMarkers(t *testing.T, body string, markers []string) {
	t.Helper()
	for _, marker := range markers {
		if !strings.Contains(body, marker) {
			t.Errorf("missing contract marker %q", marker)
		}
	}
}

func assertOrderedContractMarkers(t *testing.T, body string, markers []string) {
	t.Helper()
	previous := -1
	for _, marker := range markers {
		index := strings.Index(body, marker)
		if index < 0 {
			t.Errorf("missing ordered contract marker %q", marker)
			continue
		}
		if index <= previous {
			t.Errorf("contract marker %q appears out of order", marker)
		}
		previous = index
	}
}

func markdownH2s(body string) []string {
	var headings []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## ") {
			headings = append(headings, strings.TrimPrefix(line, "## "))
		}
	}
	return headings
}

func assertStringSlicesEqual(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s count = %d, want %d", name, len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("%s[%d] = %q, want %q", name, index, got[index], want[index])
		}
	}
}

func assertRelativeLinks(t *testing.T, readmeName, body string) {
	t.Helper()
	for _, match := range markdownRelativeLink.FindAllStringSubmatch(body, -1) {
		target := strings.SplitN(match[1], "#", 2)[0]
		if target == "" || strings.Contains(target, "://") {
			continue
		}
		fullTarget := filepath.Join(filepath.Dir(readmeName), target)
		_, err := os.Stat(fullTarget)
		if target == "../../docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.png" &&
			os.IsNotExist(err) {
			// Task 8 creates and visually verifies this one known missing asset.
			continue
		}
		if err != nil {
			t.Errorf("relative link %q from %s: %v", target, readmeName, err)
		}
	}
}
