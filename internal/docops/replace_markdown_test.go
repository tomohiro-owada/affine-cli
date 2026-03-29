package docops

import "testing"

func TestStripMirrorOfExportedDocTitle(t *testing.T) {
	title := "todo"
	cases := []struct {
		in   string
		want string
	}{
		{
			in:   "# todo\n\nhello",
			want: "hello",
		},
		{
			in:   "#todo\n\nhello",
			want: "hello",
		},
		{
			in:   "\n\n# ToDo\n\nbody",
			want: "body",
		},
		{
			in:   "# other\n\nx",
			want: "# other\n\nx",
		},
		{
			in:   "## todo\n\nx",
			want: "## todo\n\nx",
		},
		{
			in:   "hello",
			want: "hello",
		},
		{
			in:   "",
			want: "",
		},
	}
	for _, tc := range cases {
		got := stripMirrorOfExportedDocTitle(tc.in, title)
		if got != tc.want {
			t.Errorf("stripMirrorOfExportedDocTitle(%q, %q) = %q; want %q", tc.in, title, got, tc.want)
		}
	}
}

func TestStripMirrorOfExportedDocTitle_emptyTitleNoOp(t *testing.T) {
	in := "# todo\n\nx"
	if got := stripMirrorOfExportedDocTitle(in, ""); got != in {
		t.Fatalf("expected no strip when doc title empty, got %q", got)
	}
}
