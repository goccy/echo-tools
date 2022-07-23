package alp

import "testing"

func TestConvertEchoRoutes(t *testing.T) {
	testcases := []struct {
		name   string
		input  []string
		expect string
	}{
		{
			name:   "empty",
			input:  []string{},
			expect: "",
		},
		{
			name:   "simple routes",
			input:  []string{"/hoge/fuga"},
			expect: "/hoge/fuga",
		},
		{
			name:   "multiple routes",
			input:  []string{"/hoge/fuga", "/hoge/piyo"},
			expect: "/hoge/fuga,/hoge/piyo",
		},
		{
			name:   "variables",
			input:  []string{"/hoge/:id", "/hoge/piyo"},
			expect: "/hoge/.+,/hoge/piyo",
		},
		{
			name:   "variables 2",
			input:  []string{"/hoge/:id/fuga", "/hoge/piyo"},
			expect: "/hoge/.+/fuga,/hoge/piyo",
		},
		{
			name:   "asterisk",
			input:  []string{"/hoge/*", "/hoge/piyo"},
			expect: "/hoge/*,/hoge/piyo",
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			actual := ConvertEchoRoutes(tt.input)
			if actual != tt.expect {
				t.Fatalf("expect: '%s' but actual: '%s'", tt.expect, actual)
			}
		})
	}
}
