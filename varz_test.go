package router

import (
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry/go_cfmessagebus/mock_cfmessagebus"
	. "launchpad.net/gocheck"
	"net/http"
	"time"
)

type VarzSuite struct {
	Varz
	*Registry
}

var _ = Suite(&VarzSuite{})

func (s *VarzSuite) SetUpTest(c *C) {
	mbus := mock_cfmessagebus.NewMockMessageBus()
	r := NewRegistry(DefaultConfig(), mbus)
	s.Registry = r
	s.Varz = NewVarz(r)
}

// Extract value using key(s) from JSON data
// For example, when extracting value from
//       {
//         "foo": { "bar" : 1 },
//         "foobar": 2,
//        }
// findValue("foo", "bar") returns 1
// findValue("foobar") returns 2
func (s *VarzSuite) findValue(x ...string) interface{} {
	var z interface{}
	var ok bool

	b, err := json.Marshal(s.Varz)
	if err != nil {
		panic(err)
	}

	y := make(map[string]interface{})
	err = json.Unmarshal(b, &y)
	if err != nil {
		panic(err)
	}

	z = y

	for _, e := range x {
		u := z.(map[string]interface{})
		z, ok = u[e]
		if !ok {
			panic(fmt.Sprintf("no key: %s", e))
		}
	}

	return z
}

func (s *VarzSuite) TestMembersOfUniqueVarz(c *C) {
	v := s.Varz

	members := []string{
		"responses_2xx",
		"responses_3xx",
		"responses_4xx",
		"responses_5xx",
		"responses_xxx",
		"latency",
		"rate",
		"tags",
		"urls",
		"droplets",
		"requests",
		"bad_requests",
		"requests_per_sec",
		"top10_app_requests",
		"ms_since_last_registry_update",
	}

	b, e := json.Marshal(v)
	c.Assert(e, IsNil)

	d := make(map[string]interface{})
	e = json.Unmarshal(b, &d)
	c.Assert(e, IsNil)

	for _, k := range members {
		if _, ok := d[k]; !ok {
			c.Fatalf(`member "%s" not found`, k)
		}
	}
}

func (s *VarzSuite) TestSecondsSinceLastRegistryUpdate(c *C) {
	testTime := time.Now()
	time.Sleep(10 * time.Millisecond)
	s.Registry.timeOfLastUpdate = testTime
	timeSince := s.findValue("ms_since_last_registry_update").(float64)
	c.Assert(timeSince < 1000, Equals, true)
	c.Assert(timeSince >= 10, Equals, true)
}

func (s *VarzSuite) TestUrlsInVarz(c *C) {
	c.Check(s.findValue("urls"), Equals, float64(0))

	var fooReg = &RouteEndpoint{
		Host: "192.168.1.1",
		Port: 1234,
		Uris: []Uri{"foo.vcap.me", "fooo.vcap.me"},
		Tags: map[string]string{},

		ApplicationId: "12345",
	}

	// Add a route
	s.Registry.Register(fooReg)

	c.Check(s.findValue("urls"), Equals, float64(2))
}

func (s *VarzSuite) TestUpdateBadRequests(c *C) {
	r := http.Request{}

	s.CaptureBadRequest(&r)
	c.Check(s.findValue("bad_requests"), Equals, float64(1))

	s.CaptureBadRequest(&r)
	c.Check(s.findValue("bad_requests"), Equals, float64(2))
}

func (s *VarzSuite) TestUpdateRequests(c *C) {
	b := &RouteEndpoint{}
	r := http.Request{}

	s.Varz.CaptureRoutingRequest(b, &r)
	c.Check(s.findValue("requests"), Equals, float64(1))

	s.Varz.CaptureRoutingRequest(b, &r)
	c.Check(s.findValue("requests"), Equals, float64(2))
}

func (s *VarzSuite) TestUpdateRequestsWithTags(c *C) {
	b1 := &RouteEndpoint{
		Tags: map[string]string{
			"component": "cc",
		},
	}

	b2 := &RouteEndpoint{
		Tags: map[string]string{
			"component": "cc",
		},
	}

	r1 := http.Request{}
	r2 := http.Request{}

	s.Varz.CaptureRoutingRequest(b1, &r1)
	s.Varz.CaptureRoutingRequest(b2, &r2)

	c.Check(s.findValue("tags", "component", "cc", "requests"), Equals, float64(2))
}

func (s *VarzSuite) TestUpdateResponse(c *C) {
	var b *RouteEndpoint = &RouteEndpoint{}
	var d time.Duration

	r1 := &http.Response{
		StatusCode: http.StatusOK,
	}

	r2 := &http.Response{
		StatusCode: http.StatusNotFound,
	}

	s.CaptureRoutingResponse(b, r1, d)
	s.CaptureRoutingResponse(b, r2, d)
	s.CaptureRoutingResponse(b, r2, d)

	c.Check(s.findValue("responses_2xx"), Equals, float64(1))
	c.Check(s.findValue("responses_4xx"), Equals, float64(2))
}

func (s *VarzSuite) TestUpdateResponseWithTags(c *C) {
	var d time.Duration

	b1 := &RouteEndpoint{
		Tags: map[string]string{
			"component": "cc",
		},
	}

	b2 := &RouteEndpoint{
		Tags: map[string]string{
			"component": "cc",
		},
	}

	r1 := &http.Response{
		StatusCode: http.StatusOK,
	}

	r2 := &http.Response{
		StatusCode: http.StatusNotFound,
	}

	s.CaptureRoutingResponse(b1, r1, d)
	s.CaptureRoutingResponse(b2, r2, d)
	s.CaptureRoutingResponse(b2, r2, d)

	c.Check(s.findValue("tags", "component", "cc", "responses_2xx"), Equals, float64(1))
	c.Check(s.findValue("tags", "component", "cc", "responses_4xx"), Equals, float64(2))
}

func (s *VarzSuite) TestUpdateResponseLatency(c *C) {
	var routeEndpoint *RouteEndpoint = &RouteEndpoint{}
	var duration = 1 * time.Millisecond

	response := &http.Response{
		StatusCode: http.StatusOK,
	}

	s.CaptureRoutingResponse(routeEndpoint, response, duration)

	c.Check(s.findValue("latency", "50").(float64), Equals, float64(duration)/float64(time.Second))
	c.Check(s.findValue("latency", "75").(float64), Equals, float64(duration)/float64(time.Second))
	c.Check(s.findValue("latency", "90").(float64), Equals, float64(duration)/float64(time.Second))
	c.Check(s.findValue("latency", "95").(float64), Equals, float64(duration)/float64(time.Second))
	c.Check(s.findValue("latency", "99").(float64), Equals, float64(duration)/float64(time.Second))
}
