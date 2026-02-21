package ratelimit

import (
	"testing"
)

func TestCheckDestructive_UnderLimit(t *testing.T) {
	l := New(3, 3)
	for i := 0; i < 3; i++ {
		if err := l.CheckDestructive(); err != nil {
			t.Fatalf("op %d should be allowed: %v", i, err)
		}
	}
}

func TestCheckDestructive_OverLimit(t *testing.T) {
	l := New(2, 3)
	l.CheckDestructive()
	l.CheckDestructive()
	if err := l.CheckDestructive(); err == nil {
		t.Fatal("3rd op should be rejected with limit=2")
	}
}

func TestWasRolledBack(t *testing.T) {
	l := New(10, 3)
	l.RecordRollback("suse_zypper_install", "vim")

	if !l.WasRolledBack("suse_zypper_install", "vim") {
		t.Fatal("should detect recent rollback")
	}
	if l.WasRolledBack("suse_zypper_install", "git") {
		t.Fatal("should not detect rollback for different args")
	}
}

func TestLockoutAfterRollbacks(t *testing.T) {
	l := New(10, 2)
	l.RecordRollback("a", "1")
	l.RecordRollback("b", "2")

	if err := l.CheckDestructive(); err == nil {
		t.Fatal("should be locked out after 2 rollbacks (max=2)")
	}
}
