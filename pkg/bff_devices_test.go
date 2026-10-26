package nitro

import (
	"encoding/json"
	"testing"
)

func TestMobileBFF_DeviceTracking(t *testing.T) {
	auth := NewAuth(AuthConfig{JWTSecret: hardeningSecret}).WithMemoryStore().WithRateLimit(0, 0)
	cfg := NewMobileBFFConfig()
	cfg.MaxDevicesPerUser = 2
	cfg.Devices = NewMemoryDeviceStore(2)
	app := New(Config{DisableDefaults: true})
	app.MobileBFFRoutes("/m", auth.AuthUseCase, cfg, auth.Provider())

	tc := NewTestClient(app)
	reg := tc.Post(t, "/m/auth/register", `{"email":"m@example.com","password":"password123","name":"M","device":{"device_id":"d1","platform":"ios"}}`)
	tc.AssertStatus(t, reg, 201)
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.Unmarshal(reg.Body(), &tok)

	// Two more logins on new devices: limit 2 evicts the least recently seen.
	for _, id := range []string{"d2", "d3"} {
		tc.AssertStatus(t, tc.Post(t, "/m/auth/login", `{"email":"m@example.com","password":"password123","device":{"device_id":"`+id+`"}}`), 200)
	}

	authed := tc.WithHeader("Authorization", "Bearer "+tok.AccessToken)
	list := authed.Get(t, "/m/devices")
	tc.AssertStatus(t, list, 200)
	var devs []map[string]any
	if err := json.Unmarshal(list.Body(), &devs); err != nil {
		t.Fatal(err)
	}
	if len(devs) != 2 {
		t.Fatalf("devices = %d (%s), want 2 (MaxDevicesPerUser)", len(devs), list.Body())
	}
	if devs[0]["device_id"] != "d3" {
		t.Fatalf("most recent first: %s", list.Body())
	}

	authed.AssertStatus(t, authed.Delete(t, "/m/devices/d3"), 204)
	authed.AssertStatus(t, authed.Delete(t, "/m/devices/unknown"), 404)
	var after []map[string]any
	_ = json.Unmarshal(authed.Get(t, "/m/devices").Body(), &after)
	if len(after) != 1 {
		t.Fatalf("after delete: %d devices", len(after))
	}
}
