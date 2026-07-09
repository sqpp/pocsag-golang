package pocsag

import (
	"os"
	"testing"
)

func TestPOCSAGEncoding(t *testing.T) {
	// Test address encoding (verified against original C pocsag-tool)
	addressCW := EncodeAddress(123456, FuncAlphanumeric)
	expected := uint32(0x0789182E)
	if addressCW != expected {
		t.Errorf("Address codeword mismatch: got 0x%X, want 0x%X", addressCW, expected)
	}

	// Test full packet generation
	packet := CreatePOCSAGPacket(123456, "HELLO WORLD", FuncAlphanumeric)
	if len(packet) == 0 {
		t.Error("Packet generation failed")
	}

	// Test audio conversion
	wavData := ConvertToAudio(packet)
	if len(wavData) < 44 {
		t.Error("WAV file too small (missing header)")
	}

	// Verify WAV header
	if string(wavData[0:4]) != "RIFF" {
		t.Error("Invalid WAV header")
	}
	if string(wavData[8:12]) != "WAVE" {
		t.Error("Invalid WAVE marker")
	}
}

func TestBCH(t *testing.T) {
	// Test numeric address encoding (function 0) - verified against original C tool
	addrNumeric := EncodeAddress(123456, FuncNumeric)
	expectedNumeric := uint32(0x0789058B)
	if addrNumeric != expectedNumeric {
		t.Errorf("Numeric address codeword mismatch: got 0x%X, want 0x%X", addrNumeric, expectedNumeric)
	}

	// Test alphanumeric address encoding (function 3) - verified against original C tool
	addrAlpha := EncodeAddress(123456, FuncAlphanumeric)
	expectedAlpha := uint32(0x0789182E)
	if addrAlpha != expectedAlpha {
		t.Errorf("Alphanumeric address codeword mismatch: got 0x%X, want 0x%X", addrAlpha, expectedAlpha)
	}
}

func TestMessageParity(t *testing.T) {
	// Message codewords must have bit 31 set and even parity across all 32 bits
	messages := []string{"A", "HELLO", "TESTING 123"}
	for _, m := range messages {
		encoded := Ascii7BitEncoder(m)
		cws := SplitMessageIntoFrames(encoded)
		for i, cw := range cws {
			if cw&(1<<31) == 0 {
				t.Errorf("Message %q codeword %d missing bit 31", m, i)
			}
			// Count bits
			count := 0
			for bit := 0; bit < 32; bit++ {
				if cw&(1<<bit) != 0 {
					count++
				}
			}
			if count%2 != 0 {
				t.Errorf("Message %q codeword %d (0x%08X) has odd parity", m, i, cw)
			}
		}
	}
}

func TestNumericMessagesDoNotTransmitTerminator(t *testing.T) {
	cases := []string{
		"123124242",
		"123124242 212",
		"12312424",
		"12*34",
	}

	for _, message := range cases {
		packet := CreatePOCSAGPacket(1234567, message, FuncNumeric)
		decoded, err := DecodeFromBinary(packet)
		if err != nil {
			t.Fatalf("DecodeFromBinary(%q) failed: %v", message, err)
		}
		if len(decoded) != 1 {
			t.Fatalf("DecodeFromBinary(%q) got %d messages, want 1: %v", message, len(decoded), decoded)
		}
		if decoded[0].Message != message {
			t.Errorf("numeric round trip mismatch: got %q, want %q", decoded[0].Message, message)
		}
	}
}

func TestPayloadTypeIndependentFromFunctionBits(t *testing.T) {
	packet := CreatePOCSAGPacketWithPayloadType(1234567, "123124242", 1, PayloadTypeNumeric)
	decoded, err := DecodeFromBinaryWithPayloadType(packet, PayloadTypeNumeric)
	if err != nil {
		t.Fatalf("DecodeFromBinaryWithPayloadType failed: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("got %d messages, want 1: %v", len(decoded), decoded)
	}
	if decoded[0].Function != 1 {
		t.Fatalf("function mismatch: got %d, want 1", decoded[0].Function)
	}
	if !decoded[0].IsNumeric || decoded[0].Message != "123124242" {
		t.Fatalf("payload mismatch: got numeric=%v message=%q", decoded[0].IsNumeric, decoded[0].Message)
	}
}

func TestExample(t *testing.T) {
	// Generate example file like the C tool
	packet := CreatePOCSAGPacket(4444, "Broadcast this on hackrf", FuncAlphanumeric)
	wavData := ConvertToAudio(packet)

	err := os.WriteFile("example.wav", wavData, 0644)
	if err != nil {
		t.Fatalf("Failed to write example.wav: %v", err)
	}

	t.Log("✅ Generated example.wav")
}
