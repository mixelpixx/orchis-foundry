package api

import "testing"

func TestCanWriteCanAdmin(t *testing.T) {
	cases := []struct {
		role        string
		write, admin bool
	}{
		{roleOwner, true, true},
		{roleAdmin, true, true},
		{roleWrite, true, false},
		{roleRead, false, false},
		{"", false, false},
	}
	for _, c := range cases {
		row := &repoRow{Role: c.role}
		if got := canWrite(row); got != c.write {
			t.Errorf("canWrite(%q) = %v, want %v", c.role, got, c.write)
		}
		if got := canAdmin(row); got != c.admin {
			t.Errorf("canAdmin(%q) = %v, want %v", c.role, got, c.admin)
		}
	}
}
