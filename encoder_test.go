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
