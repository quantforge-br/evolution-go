package nats_producer

import "testing"

func TestEmptyURLDoesNotConnectToDefaultNATSServer(t *testing.T) {
	producer := NewNatsProducer("", true, []string{"message"}, nil).(*natsProducer)
	if producer.conn != nil || producer.natsGlobalEnabled {
		t.Fatal("an empty URL must disable NATS without opening a connection")
	}
}
