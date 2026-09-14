package ds_test

import (
	"slices"
	"testing"

	"github.com/azuiktech/kleisli-go/ds"
)

type TestUser struct {
	ID      int64
	Email   string
	ZipCode uint32
	Note    string // not part of any index
}

var (
	testByID      = ds.UniqueIndex(func(u *TestUser) int64 { return u.ID })
	testByEmail   = ds.UniqueIndex(func(u *TestUser) string { return u.Email })
	testByZipCode = ds.NonUniqueIndex(func(u *TestUser) uint32 { return u.ZipCode })
)

func newSampleTable() *ds.Table[TestUser] {
	return ds.NewTable(testByID, testByEmail, testByZipCode)
}

func TestTable_InsertAndQuery(t *testing.T) {
	tbl := newSampleTable()

	u1 := &TestUser{ID: 1, Email: "alice@example.com", ZipCode: 94016}
	u2 := &TestUser{ID: 2, Email: "bob@example.com", ZipCode: 94016}
	u3 := &TestUser{ID: 3, Email: "charlie@example.com", ZipCode: 10001}

	if !tbl.Insert(u1) {
		t.Fatal("expected insert u1 to succeed")
	}
	if !tbl.Insert(u2) {
		t.Fatal("expected insert u2 to succeed")
	}
	if !tbl.Insert(u3) {
		t.Fatal("expected insert u3 to succeed")
	}

	if got := tbl.Len(); got != 3 {
		t.Fatalf("expected Len 3, got %d", got)
	}

	// Query unique index
	opt1 := testByID.From(tbl).Find(1)
	if !opt1.IsSome() {
		t.Fatal("expected to find user with ID 1")
	}
	if opt1.MustGet().Email != "alice@example.com" {
		t.Fatalf("expected alice@example.com, got %s", opt1.MustGet().Email)
	}

	// Query non-existing unique index
	opt404 := testByID.From(tbl).Find(999)
	if opt404.IsSome() {
		t.Fatal("expected None for non-existing ID 999")
	}

	// Query non-unique index with multiple matches
	matches94016 := testByZipCode.From(tbl).Find(94016)
	if len(matches94016) != 2 {
		t.Fatalf("expected 2 users in zip 94016, got %d", len(matches94016))
	}
	emails := []string{matches94016[0].Email, matches94016[1].Email}
	slices.Sort(emails)
	if !slices.Equal(emails, []string{"alice@example.com", "bob@example.com"}) {
		t.Fatalf("unexpected emails in zip 94016: %v", emails)
	}

	// Query non-unique index with single match
	matches10001 := testByZipCode.From(tbl).Find(10001)
	if len(matches10001) != 1 || matches10001[0].Email != "charlie@example.com" {
		t.Fatalf("unexpected result for zip 10001: %v", matches10001)
	}

	// Query non-unique index with no matches
	matches00000 := testByZipCode.From(tbl).Find(0)
	if len(matches00000) != 0 {
		t.Fatalf("expected 0 matches for zip 0, got %d", len(matches00000))
	}
}

func TestTable_UniqueConstraintEnforcement(t *testing.T) {
	tbl := newSampleTable()

	u1 := &TestUser{ID: 1, Email: "alice@example.com", ZipCode: 94016}
	if !tbl.Insert(u1) {
		t.Fatal("insert u1 should succeed")
	}

	// Duplicate ID (primary unique conflict)
	u2DuplicateID := &TestUser{ID: 1, Email: "other@example.com", ZipCode: 90210}
	if tbl.Insert(u2DuplicateID) {
		t.Fatal("insert with duplicate ID must return false")
	}

	// Duplicate Email (secondary unique conflict)
	u3DuplicateEmail := &TestUser{ID: 2, Email: "alice@example.com", ZipCode: 90210}
	if tbl.Insert(u3DuplicateEmail) {
		t.Fatal("insert with duplicate Email must return false")
	}

	// Ensure atomic rollback: failed inserts must not leave partial index entries
	if got := tbl.Len(); got != 1 {
		t.Fatalf("expected Len 1 after failed inserts, got %d", got)
	}
	if testByID.From(tbl).Exists(2) {
		t.Fatal("ID 2 must not exist in index after rollback")
	}
	if len(testByZipCode.From(tbl).Find(90210)) != 0 {
		t.Fatal("zip 90210 must have 0 entries after rollback")
	}
}

func TestTable_Delete(t *testing.T) {
	tbl := newSampleTable()

	u1 := &TestUser{ID: 1, Email: "alice@example.com", ZipCode: 94016}
	u2 := &TestUser{ID: 2, Email: "bob@example.com", ZipCode: 94016}
	tbl.Insert(u1)
	tbl.Insert(u2)

	// Delete existing user
	if !tbl.Delete(u1) {
		t.Fatal("expected Delete(u1) to return true")
	}

	if tbl.Len() != 1 {
		t.Fatalf("expected Len 1 after deleting u1, got %d", tbl.Len())
	}

	// Verify all indexes were cleaned up
	if testByID.From(tbl).Exists(1) {
		t.Fatal("ID 1 should not exist after deletion")
	}
	if testByEmail.From(tbl).Exists("alice@example.com") {
		t.Fatal("alice@example.com should not exist after deletion")
	}

	matches := testByZipCode.From(tbl).Find(94016)
	if len(matches) != 1 || matches[0].ID != 2 {
		t.Fatalf("expected only bob (ID 2) in zip 94016, got %v", matches)
	}

	// Delete non-existing returns false
	uNonExistent := &TestUser{ID: 99, Email: "ghost@example.com", ZipCode: 12345}
	if tbl.Delete(uNonExistent) {
		t.Fatal("deleting non-existent user should return false")
	}

	// Re-inserting deleted key should now succeed
	u1New := &TestUser{ID: 1, Email: "alice.new@example.com", ZipCode: 30301}
	if !tbl.Insert(u1New) {
		t.Fatal("expected re-inserting ID 1 to succeed after deletion")
	}
	if tbl.Len() != 2 {
		t.Fatalf("expected Len 2, got %d", tbl.Len())
	}
}

func TestTable_Clear(t *testing.T) {
	tbl := newSampleTable()

	tbl.Insert(&TestUser{ID: 1, Email: "alice@example.com", ZipCode: 94016})
	tbl.Insert(&TestUser{ID: 2, Email: "bob@example.com", ZipCode: 94016})

	tbl.Clear()

	if tbl.Len() != 0 {
		t.Fatalf("expected Len 0 after Clear, got %d", tbl.Len())
	}
	if testByID.From(tbl).Exists(1) {
		t.Fatal("ID 1 should not exist after Clear")
	}
	if len(testByZipCode.From(tbl).Find(94016)) != 0 {
		t.Fatal("Zip 94016 should be empty after Clear")
	}
}

func TestTable_QuerySyntaxes(t *testing.T) {
	tbl := newSampleTable()

	u := &TestUser{ID: 42, Email: "douglas@galaxy.org", ZipCode: 42424}
	tbl.Insert(u)

	// Syntax 1: ByID.Find(tbl, 42)
	s1 := testByID.Find(tbl, 42)
	if !s1.IsSome() || s1.MustGet().ID != 42 {
		t.Fatal("Syntax 1 ByID.Find failed")
	}

	// Syntax 2: ByID.From(tbl).Find(42)
	s2 := testByID.From(tbl).Find(42)
	if !s2.IsSome() || s2.MustGet().ID != 42 {
		t.Fatal("Syntax 2 ByID.From(tbl).Find failed")
	}

	// Syntax 3: ds.By(tbl, ByID).Find(42)
	s3 := ds.By(tbl, testByID).Find(42)
	if !s3.IsSome() || s3.MustGet().ID != 42 {
		t.Fatal("Syntax 3 ds.By(tbl, ByID).Find failed")
	}

	// Non-unique syntaxes
	nu1 := testByZipCode.Find(tbl, 42424)
	if len(nu1) != 1 || nu1[0].ID != 42 {
		t.Fatal("Non-unique Syntax 1 failed")
	}

	nu2 := testByZipCode.From(tbl).Find(42424)
	if len(nu2) != 1 || nu2[0].ID != 42 {
		t.Fatal("Non-unique Syntax 2 failed")
	}

	nu3 := ds.ByGroup(tbl, testByZipCode).Find(42424)
	if len(nu3) != 1 || nu3[0].ID != 42 {
		t.Fatal("Non-unique Syntax 3 failed")
	}
}

func TestTable_IntegrationMultiIndexLifecycle(t *testing.T) {
	tbl := newSampleTable()

	// 100 users across 5 zip codes
	for i := int64(1); i <= 100; i++ {
		u := &TestUser{
			ID:      i,
			Email:   "user@example.com", // will conflict on second insert!
			ZipCode: uint32(i % 5),
		}
		if i == 1 {
			if !tbl.Insert(u) {
				t.Fatalf("failed inserting first user: %d", i)
			}
		} else {
			// Email is duplicate, must fail
			if tbl.Insert(u) {
				t.Fatalf("expected duplicate email conflict for user: %d", i)
			}
		}
	}

	if tbl.Len() != 1 {
		t.Fatalf("expected Len 1, got %d", tbl.Len())
	}
}

func TestTable_PrimaryKeyMainStorage(t *testing.T) {
	// Verify that NewTable correctly takes testByID as PK and secondary indexes
	tbl := ds.NewTable(testByID, testByEmail, testByZipCode)

	u1 := &TestUser{ID: 10, Email: "user10@example.com", ZipCode: 50001}
	u2 := &TestUser{ID: 20, Email: "user20@example.com", ZipCode: 50001}

	tbl.Insert(u1)
	tbl.Insert(u2)

	if tbl.Len() != 2 {
		t.Fatalf("expected Len 2, got %d", tbl.Len())
	}

	// Delete u1 directly and verify main table length decreases immediately
	if !tbl.Delete(u1) {
		t.Fatal("expected Delete(u1) to return true")
	}
	if tbl.Len() != 1 {
		t.Fatalf("expected Len 1 after delete, got %d", tbl.Len())
	}

	// Re-insert u1 with new email
	u1Re := &TestUser{ID: 10, Email: "user10.new@example.com", ZipCode: 50002}
	if !tbl.Insert(u1Re) {
		t.Fatal("expected re-insert of ID 10 to succeed")
	}
	if tbl.Len() != 2 {
		t.Fatalf("expected Len 2 after re-insert, got %d", tbl.Len())
	}

	got := testByID.From(tbl).Find(10)
	if !got.IsSome() || got.MustGet().Email != "user10.new@example.com" {
		t.Fatalf("expected updated email, got %v", got)
	}
}

func TestTable_All(t *testing.T) {
	tbl := newSampleTable()
	u1 := &TestUser{ID: 1, Email: "a@ex.com", ZipCode: 100}
	u2 := &TestUser{ID: 2, Email: "b@ex.com", ZipCode: 200}
	u3 := &TestUser{ID: 3, Email: "c@ex.com", ZipCode: 300}

	tbl.Insert(u1)
	tbl.Insert(u2)
	tbl.Insert(u3)

	var all []*TestUser
	for u := range tbl.All() {
		all = append(all, u)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 items from All(), got %d", len(all))
	}

	// Verify early break from iterator
	count := 0
	for range tbl.All() {
		count++
		break
	}
	if count != 1 {
		t.Fatalf("expected early break to stop after 1 iteration, got %d", count)
	}
}

func TestTable_Update_Success(t *testing.T) {
	tbl := newSampleTable()
	u1 := &TestUser{ID: 1, Email: "alice@ex.com", ZipCode: 94016}
	tbl.Insert(u1)

	// Update secondary non-unique index and unique index
	ok := tbl.Update(u1, func(u *TestUser) {
		u.Email = "alice.new@ex.com"
		u.ZipCode = 90210
	})
	if !ok {
		t.Fatal("expected Update to succeed")
	}

	// Old email should not exist
	if testByEmail.From(tbl).Find("alice@ex.com").IsSome() {
		t.Fatal("old email should no longer be indexed")
	}
	// New email should exist
	if !testByEmail.From(tbl).Find("alice.new@ex.com").IsSome() {
		t.Fatal("new email should be indexed")
	}
	// Old zip code should have count 0
	if testByZipCode.From(tbl).Count(94016) != 0 {
		t.Fatal("old zipcode should have 0 items")
	}
	// New zip code should have count 1
	if testByZipCode.From(tbl).Count(90210) != 1 {
		t.Fatal("new zipcode should have 1 item")
	}
}

func TestTable_Update_ConstraintViolation_Rollback(t *testing.T) {
	tbl := newSampleTable()
	u1 := &TestUser{ID: 1, Email: "alice@ex.com", ZipCode: 100}
	u2 := &TestUser{ID: 2, Email: "bob@ex.com", ZipCode: 200}
	tbl.Insert(u1)
	tbl.Insert(u2)

	// Attempt to change u1's email to u2's email (unique constraint violation)
	ok := tbl.Update(u1, func(u *TestUser) {
		u.Email = "bob@ex.com"
		u.ZipCode = 999
	})
	if ok {
		t.Fatal("expected Update to fail due to email collision")
	}

	// u1 should have rolled back to original state
	if u1.Email != "alice@ex.com" {
		t.Fatalf("expected email rolled back to alice@ex.com, got %s", u1.Email)
	}
	if u1.ZipCode != 100 {
		t.Fatalf("expected zip rolled back to 100, got %d", u1.ZipCode)
	}

	// Indexes must still be intact
	if !testByEmail.From(tbl).Find("alice@ex.com").IsSome() {
		t.Fatal("alice@ex.com must still be indexed")
	}
	if !testByEmail.From(tbl).Find("bob@ex.com").IsSome() {
		t.Fatal("bob@ex.com must still be indexed")
	}
	if testByZipCode.From(tbl).Count(100) != 1 {
		t.Fatal("zipcode 100 must still have 1 item")
	}
	if testByZipCode.From(tbl).Count(999) != 0 {
		t.Fatal("zipcode 999 must have 0 items")
	}
}

func TestTable_Update_EdgeCases(t *testing.T) {
	tbl := newSampleTable()
	u1 := &TestUser{ID: 1, Email: "alice@ex.com", ZipCode: 100}

	// Not in table
	if tbl.Update(u1, func(u *TestUser) { u.ZipCode = 200 }) {
		t.Fatal("expected Update on uninserted record to return false")
	}

	tbl.Insert(u1)
	// Nil checks
	if tbl.Update(nil, func(u *TestUser) {}) {
		t.Fatal("expected Update(nil, ...) to return false")
	}
	if tbl.Update(u1, nil) {
		t.Fatal("expected Update(u1, nil) to return false")
	}
}

func TestTable_Update_NonIndexedField(t *testing.T) {
	tbl := newSampleTable()
	u1 := &TestUser{ID: 1, Email: "alice@ex.com", ZipCode: 94016, Note: "original"}
	tbl.Insert(u1)

	ok := tbl.Update(u1, func(u *TestUser) {
		u.Note = "updated"
	})
	if !ok {
		t.Fatal("expected Update mutating a non-indexed field to succeed")
	}
	if u1.Note != "updated" {
		t.Fatalf("expected Note to be updated, got %s", u1.Note)
	}

	// Index memberships must be unchanged
	if !testByID.From(tbl).Exists(1) {
		t.Fatal("ID 1 should still be indexed after non-indexed field update")
	}
	if !testByEmail.From(tbl).Exists("alice@ex.com") {
		t.Fatal("email should still be indexed after non-indexed field update")
	}
	if testByZipCode.From(tbl).Count(94016) != 1 {
		t.Fatal("zipcode 94016 should still have 1 item after non-indexed field update")
	}
	if tbl.Len() != 1 {
		t.Fatalf("expected Len 1, got %d", tbl.Len())
	}
}

func TestTable_NonUniqueView_Find_AbsentKey(t *testing.T) {
	tbl := newSampleTable()
	tbl.Insert(&TestUser{ID: 1, Email: "alice@ex.com", ZipCode: 94016})

	matches := testByZipCode.From(tbl).Find(99999)
	if matches != nil {
		t.Fatalf("Find on absent key want nil, got %v", matches)
	}
	if len(matches) != 0 {
		t.Fatalf("Find on absent key want 0 matches, got %d", len(matches))
	}
}

func TestTable_All_Empty(t *testing.T) {
	tbl := newSampleTable()

	count := 0
	for range tbl.All() {
		count++
	}
	if count != 0 {
		t.Fatalf("expected 0 iterations for All() on freshly constructed empty table, got %d", count)
	}
}

func TestTable_NonUniqueStorage_Len(t *testing.T) {
	tbl := newSampleTable()
	u1 := &TestUser{ID: 1, Email: "alice@ex.com", ZipCode: 94016}
	u2 := &TestUser{ID: 2, Email: "bob@ex.com", ZipCode: 94016}
	u3 := &TestUser{ID: 3, Email: "charlie@ex.com", ZipCode: 10001}

	tbl.Insert(u1)
	tbl.Insert(u2)
	tbl.Insert(u3)

	if testByZipCode.From(tbl).Count(94016) != 2 {
		t.Fatalf("expected count 2 for 94016, got %d", testByZipCode.From(tbl).Count(94016))
	}
	if testByZipCode.From(tbl).Count(10001) != 1 {
		t.Fatalf("expected count 1 for 10001, got %d", testByZipCode.From(tbl).Count(10001))
	}

	tbl.Delete(u1)
	if testByZipCode.From(tbl).Count(94016) != 1 {
		t.Fatalf("expected count 1 for 94016 after delete, got %d", testByZipCode.From(tbl).Count(94016))
	}

	tbl.Clear()
	if testByZipCode.From(tbl).Count(94016) != 0 {
		t.Fatalf("expected count 0 for 94016 after clear, got %d", testByZipCode.From(tbl).Count(94016))
	}
}
