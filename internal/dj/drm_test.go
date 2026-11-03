package dj

import "testing"

// The token is pinned to reference values the Edge client itself makes,
// so a slip in the port cannot ship quietly.
func TestSecMSGEC(t *testing.T) {
	cases := []struct {
		ts   float64
		want string
	}{
		{1762140000, "32935BBAB22CB819-966C89F1A8C53C7265591D22981DC5154C5729B96558DD220DD319096161198F"},
		{0, "52BC7A17B50DFEC9-28C4394775C8751CD2F6859C030C06DB1489ACAB3B3A7651831FC26895177ED6"},
		{1762145999.7, "EBF306DF5C7BDD9E-C7FD24833914E493DA1399ADD8249753C93AA18B6229409EB252366499AA5AA6"},
	}
	for _, c := range cases {
		if got := secMSGECAt(c.ts); got != c.want {
			t.Errorf("secMSGECAt(%v) =\n  %s\nwant\n  %s", c.ts, got, c.want)
		}
	}
}
