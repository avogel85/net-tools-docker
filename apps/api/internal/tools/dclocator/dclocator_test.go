package dclocator

import (
	"net/url"
	"slices"
	"testing"

	"github.com/digibaro/net-tools-docker/apps/api/internal/dnsx/dnstest"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

func zone(t *testing.T) string {
	addr := dnstest.Zone(t,
		"_ldap._tcp.corp.test. 600 IN SRV 0 100 389 dc1.corp.test.",
		"_ldap._tcp.corp.test. 600 IN SRV 0 100 389 dc2.corp.test.",
		"_ldap._tcp.dc._msdcs.corp.test. 600 IN SRV 0 100 389 dc1.corp.test.",
		"_ldap._tcp.dc._msdcs.corp.test. 600 IN SRV 0 100 389 dc2.corp.test.",
		"_ldap._tcp.pdc._msdcs.corp.test. 600 IN SRV 0 100 389 dc1.corp.test.",
		"_ldap._tcp.gc._msdcs.corp.test. 600 IN SRV 0 100 3268 dc1.corp.test.",
		"_kerberos._tcp.corp.test. 600 IN SRV 0 100 88 dc1.corp.test.",
		"_kerberos._tcp.corp.test. 600 IN SRV 0 100 88 dc2.corp.test.",
		"_kpasswd._tcp.corp.test. 600 IN SRV 0 100 4464 dc1.corp.test.",
		"_ldap._tcp.hq._sites.dc._msdcs.corp.test. 600 IN SRV 0 100 389 dc1.corp.test.",
		"dc1.corp.test. 600 IN A 10.0.0.11",
		"dc1.corp.test. 600 IN AAAA fd00::11",
		"_ldap._tcp.broken.test. 600 IN SRV 0 100 389 gone.broken.test.",
		"_ldap._tcp.dc._msdcs.broken.test. 600 IN SRV 0 100 389 gone.broken.test.",
	)
	return "&resolver=" + url.QueryEscape(addr.String())
}

func TestLocate(t *testing.T) {
	q := zone(t)
	env := tooltest.New(t, &Tool{}, tooltest.Options{AllowPrivate: true})
	var r Result
	env.Get("/api/v1/dns/dc-locator?domain=CORP.test&site=hq"+q).Must(t).Decode(t, &r)
	if r.Domain != "corp.test" || len(r.Lookups) != 14 {
		t.Fatalf("result = %+v", r)
	}
	if len(r.DCs) != 2 || r.DCs[0].Name != "dc1.corp.test" || !slices.Equal(r.DCs[0].Addresses, []string{"10.0.0.11", "fd00::11"}) {
		t.Errorf("dcs = %+v", r.DCs)
	}
	if !slices.Contains(r.DCs[0].Services, "pdc") || slices.Contains(r.DCs[1].Services, "pdc") {
		t.Errorf("services = %+v", r.DCs)
	}
	// dc2 has no address; kpasswd on an unexpected port; site lookup found.
	if !slices.Contains(r.Problems, "target-no-address") || !slices.Contains(r.Warnings, "unexpected-port") || slices.Contains(r.Problems, "missing-site-ldap") {
		t.Errorf("problems %v warnings %v", r.Problems, r.Warnings)
	}

	env.Get("/api/v1/dns/dc-locator?domain=broken.test"+q).Must(t).Decode(t, &r)
	if !slices.Contains(r.Problems, "missing-kerberos-tcp") || !slices.Contains(r.Warnings, "missing-pdc") {
		t.Errorf("broken: problems %v warnings %v", r.Problems, r.Warnings)
	}
	env.Get("/api/v1/dns/dc-locator?domain=none.test"+q).Must(t).Decode(t, &r)
	if !slices.Contains(r.Problems, "missing-dc") || len(r.DCs) != 0 {
		t.Errorf("none: %+v", r)
	}
}

func TestValidation(t *testing.T) {
	env := tooltest.New(t, &Tool{}, tooltest.Options{})
	env.Get("/api/v1/dns/dc-locator?domain=nodot").ExpectError(t, "INVALID_INPUT")
	env.Get("/api/v1/dns/dc-locator?domain=corp.test&site=a.b").ExpectError(t, "INVALID_INPUT")
	env.Get("/api/v1/dns/dc-locator?domain=corp.test&resolver=10.0.0.53").ExpectError(t, "TARGET_PRIVATE_DENIED")
}
