package config

import "testing"

func TestWhatsAppAutoVersionRequiresExplicitOptIn(t *testing.T) {
	t.Setenv("POSTGRES_USERS_DB", "postgresql://local:local@localhost/local")
	t.Setenv("DATABASE_SAVE_MESSAGES", "false")
	t.Setenv("GLOBAL_API_KEY", "private-test-key")
	t.Setenv("AMQP_URL", "")
	for _, value := range []string{"", "false", "true"} {
		t.Run("value="+value, func(t *testing.T) {
			t.Setenv("WHATSAPP_AUTO_VERSION", value)
			if got := Load().WhatsAppAutoVersion; got != (value == "true") {
				t.Fatalf("WHATSAPP_AUTO_VERSION=%q produced %v", value, got)
			}
		})
	}
}
