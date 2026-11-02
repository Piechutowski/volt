package naming

import "testing"

func TestGoNames(t *testing.T) {
	cases := []struct{ in, want string }{
		{"user_id", "UserID"},
		{"api_key", "APIKey"},
		{"http_status_url", "HTTPStatusURL"},
		{"uuid", "UUID"},
		{"full name", "FullName"},
		{"_2fa_codes", "X2faCodes"},
		{"created_at", "CreatedAt"},
		{"id", "ID"},
		{"żółć", "Żółć"},
	}
	for _, tc := range cases {
		got, err := GoName(tc.in)
		if err != nil {
			t.Errorf("GoName(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("GoName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if _, err := GoName("__"); err == nil {
		t.Error("GoName(__): want error")
	}
}

func TestArgNames(t *testing.T) {
	cases := []struct{ in, want string }{
		{"ID", "id"},
		{"UserID", "userID"},
		{"APIKey", "apiKey"},
		{"HTTPStatus", "httpStatus"},
		{"Name", "name"},
		{"X2faCode", "x2faCode"},
		{"Type", "type_"},
		{"Ctx", "ctx_"},
	}
	for _, tc := range cases {
		if got := ArgName(tc.in); got != tc.want {
			t.Errorf("ArgName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestParamNamesAvoidLocals pins the parameter a key column takes in a
// generated method (spec §generated-query-surface): never rt, nil, append
// or a name the route files' signatures take, and never a name of the form
// the data generator gives its own locals and helpers.
func TestParamNamesAvoidLocals(t *testing.T) {
	cases := []struct{ column, want string }{
		{"err", "err"},
		{"rt", "rt_"},
		{"volt_err", "voltErr_"},
		{"volt_rows", "voltRows_"},
		{"volt_post_scan", "voltPostScan_"},
		{"voltage", "voltage"},
		{"volt", "volt_"},
		{"q", "q_"},
		{"nil", "nil_"},
		{"append", "append_"},
		{"len", "len"},
		{"post_scan", "postScan"},
		{"c", "c_"},
		{"w", "w_"},
		{"r", "r_"},
		{"cs", "cs_"},
		{"opts", "opts_"},
		{"ctx", "ctx_"},
	}
	for _, tc := range cases {
		goField, err := GoName(tc.column)
		if err != nil {
			t.Fatal(err)
		}
		got := paramName(goField)
		if got != tc.want {
			t.Errorf("column %q: parameter %q, want %q", tc.column, got, tc.want)
		}
		if LocalName(got) {
			t.Errorf("column %q: parameter %q has the form of a generated local", tc.column, got)
		}
	}
	for name, want := range map[string]bool{"voltErr": true, "voltV": true, "volt": false, "voltage": false, "volt_": false, "err": false} {
		if got := LocalName(name); got != want {
			t.Errorf("LocalName(%q) = %v, want %v", name, got, want)
		}
	}
}
