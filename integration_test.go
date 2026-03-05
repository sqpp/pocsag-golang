package pocsag

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runWSLMultimon runs multimon-ng inside WSL on the given wav file
func runWSLMultimon(t *testing.T, wavPath string, baud int) string {
	demod := fmt.Sprintf("POCSAG%d", baud)
	var cmd *exec.Cmd

	// Get absolute path to the wav file
	absPath, _ := filepath.Abs(wavPath)

	if os.Getenv("WSL_DISTRO_NAME") != "" || strings.Contains(strings.ToLower(os.Getenv("PATH")), "linux") {
		// We are already inside WSL or Linux
		cmd = exec.Command("multimon-ng", "-t", "wav", "-a", demod, absPath)
	} else {
		// We are on Windows, call via wsl
		// Convert Windows path to WSL path (/mnt/e/...)
		wslPath := "/mnt/e/Development/Apps/pocsag-golang/" + filepath.ToSlash(wavPath)
		cmd = exec.Command("wsl", "multimon-ng", "-t", "wav", "-a", demod, wslPath)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("multimon-ng failed or returned error: %v\nOutput: %s", err, string(output))
		// Some versions of multimon-ng exit with 1 on success if sox warnings occur
	}

	return string(output)
}

func TestIntegration(t *testing.T) {
	bauds := []int{512, 1200, 2400}

	testFolder := "test_output"
	os.MkdirAll(testFolder, 0755)

	type testCase struct {
		name      string
		address   uint32
		message   string
		function  uint8
		baud      int
		encrypted bool
		password  string
	}

	var testCases []testCase

	for _, baud := range bauds {
		// Alphanumeric cases
		testCases = append(testCases, testCase{
			name:     fmt.Sprintf("%d_Alpha", baud),
			address:  123456,
			message:  fmt.Sprintf("TEST %d ALPHA", baud),
			function: FuncAlphanumeric,
			baud:     baud,
		})

		// Numeric cases
		testCases = append(testCases, testCase{
			name:     fmt.Sprintf("%d_Numeric", baud),
			address:  654321,
			message:  "0123456789",
			function: FuncNumeric,
			baud:     baud,
		})

		// Encrypted cases
		testCases = append(testCases, testCase{
			name:      fmt.Sprintf("%d_Encrypted", baud),
			address:   555555,
			message:   fmt.Sprintf("SECRET %d", baud),
			function:  FuncAlphanumeric,
			baud:      baud,
			encrypted: true,
			password:  "pocsag-pass-123",
		})
	}

	// Add specific long message cases to verify DPLL synchronization over time
	longMsg := "THIS IS A VERY LONG MESSAGE DESIGNED TO TEST THE STABILITY OF THE DPLL BIT-CLOCK RECOVERY LOGIC OVER AN EXTENDED TRANSMISSION PERIOD. " +
		"WE WANT TO ENSURE THAT THE SAMPLING PHASE DOES NOT DRIFT AWAY FROM THE BIT CENTERS EVEN AFTER SEVERAL HUNDRED BITS HAVE BEEN PROCESSED. " +
		"IF THE CLOCK IS STABLE, BOTH THE INTERNAL DECODER AND MULTIMON-NG SHOULD BE ABLE TO DECODE THIS ENTIRE STRING WITHOUT A SINGLE CHARACTER ERROR. " +
		"BY REPEATING THIS TEXT WE CAN REACH A LENGTH THAT TRULY CHALLENGES THE TIMING ACCURACY FOR ALL SUPPORTED BAUD RATES."

	for _, baud := range bauds {
		testCases = append(testCases, testCase{
			name:     fmt.Sprintf("%d_Long_Message", baud),
			address:  888888 + uint32(baud),
			message:  longMsg,
			function: FuncAlphanumeric,
			baud:     baud,
		})
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wavName := fmt.Sprintf("%s.wav", tc.name)
			wavPath := filepath.Join(testFolder, wavName)

			var packet []byte
			var err error

			if tc.encrypted {
				config := EncryptionConfig{
					Method: EncryptionAES256,
					Key:    []byte(tc.password),
				}
				packet, err = CreatePOCSAGPacketWithEncryption(tc.address, tc.message, tc.function, tc.baud, config)
				if err != nil {
					t.Fatalf("Failed to create encrypted packet: %v", err)
				}
			} else {
				packet = CreatePOCSAGPacketWithBaudRate(tc.address, tc.message, tc.function, tc.baud)
			}

			wavData := ConvertToAudioWithBaudRate(packet, tc.baud)
			err = os.WriteFile(wavPath, wavData, 0644)
			if err != nil {
				t.Fatalf("Failed to write WAV: %v", err)
			}

			// 1. Internal Decode Verification
			var decoded []DecodedMessage
			if tc.encrypted {
				config := EncryptionConfig{
					Method: EncryptionAES256,
					Key:    []byte(tc.password),
				}
				decoded, err = DecodeFromAudioWithDecryption(wavData, tc.baud, config)
			} else {
				decoded, err = DecodeFromAudioWithBaudRate(wavData, tc.baud)
			}

			if err != nil {
				t.Errorf("Internal decoder failed: %v", err)
			} else if len(decoded) == 0 {
				t.Errorf("Internal decoder found zero messages")
			} else {
				found := false
				for _, m := range decoded {
					if m.Address == tc.address && m.Message == tc.message {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Internal decode mismatch. Expected %d:%s, got %v", tc.address, tc.message, decoded)
				}
			}

			// 2. multimon-ng Verification (skip message content check for encryption as it shows base64)
			multimonOutput := runWSLMultimon(t, wavPath, tc.baud)
			if !tc.encrypted {
				if !strings.Contains(multimonOutput, tc.message) {
					t.Errorf("multimon-ng failed to find message in output:\n%s", multimonOutput)
				}
			}
			if !strings.Contains(multimonOutput, fmt.Sprintf("%d", (tc.address/8)*8)) && !strings.Contains(multimonOutput, fmt.Sprintf("%d", tc.address)) {
				t.Errorf("multimon-ng address mismatch in output:\n%s", multimonOutput)
			}
		})
	}
}

func TestBurstIntegration(t *testing.T) {
	messages := []MessageInfo{
		{Address: 111111, Message: "MSG 1", Function: FuncAlphanumeric},
		{Address: 222222, Message: "MSG 2", Function: FuncAlphanumeric},
		{Address: 333333, Message: "987654321", Function: FuncNumeric},
	}

	baud := 1200
	packet := CreatePOCSAGBurstWithBaudRate(messages, baud)
	wavData := ConvertToAudioWithBaudRate(packet, baud)

	testFolder := "test_output"
	wavPath := filepath.Join(testFolder, "BurstTest.wav")
	os.WriteFile(wavPath, wavData, 0644)

	// Internal Decode
	decoded, err := DecodeFromAudioWithBaudRate(wavData, baud)
	if err != nil {
		t.Fatalf("Burst internal decode failed: %v", err)
	}

	if len(decoded) != len(messages) {
		t.Errorf("Burst count mismatch: got %d, want %d", len(decoded), len(messages))
	}

	// multimon-ng
	multimonOutput := runWSLMultimon(t, wavPath, baud)
	for _, m := range messages {
		if !strings.Contains(multimonOutput, m.Message) {
			t.Errorf("multimon-ng missed burst message: %s", m.Message)
		}
	}
}
