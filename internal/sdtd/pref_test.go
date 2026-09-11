package sdtd

import (
	"context"
	"testing"
)

func TestIntegrationReadGamePref(t *testing.T) {
	c := liveClient(t)
	v, err := c.ReadGamePref(context.Background(), "AirDropFrequency")
	if err != nil {
		t.Fatalf("ReadGamePref: %v", err)
	}
	t.Logf("AirDropFrequency = %q", v)

	if _, err := c.ReadGamePref(context.Background(), "NoSuchPrefAtAll"); err == nil {
		t.Error("ReadGamePref succeeded for a name that does not exist")
	}
}
