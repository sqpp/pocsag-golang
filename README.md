# POCSAG-GO v2.3.5

A complete Go implementation of the POCSAG pager protocol — encoder, decoder, and everything in between. Ported from [pocsag-tool](https://github.com/hazardousfirmware/pocsag-tool) and extended significantly.

## What it can do

- Encode POCSAG messages as WAV audio files (512, 1200, or 2400 baud)
- Decode those WAV files back to text, with a bit-aware sync engine that handles real-world recordings
- BCH(31,21) error correction and parity validation on every codeword
- Full 21-bit RIC/capcode addressing, placed in the correct frame per ITU-R M.584-2
- Numeric (BCD) and 7-bit ASCII alphanumeric message types
- AES-256/AES-128 encryption with password-based key derivation
- Burst mode — pack multiple messages for different pagers into one WAV
- GPU-accelerated waterfall spectrogram (OpenGL 4.1) with PNG export
- JSON output for scripting and API integration
- Works out of the box with `multimon-ng` and `pocsag-decode`

---

## Installation

```bash
# Encoder
go install github.com/sqpp/pocsag-golang/v2/cmd/pocsag@latest

# Decoder
go install github.com/sqpp/pocsag-golang/v2/cmd/pocsag-decode@latest

# Burst encoder (multiple messages at once)
go install github.com/sqpp/pocsag-golang/v2/cmd/pocsag-burst@latest
```

Or build from source:

```bash
git clone https://github.com/sqpp/pocsag-golang.git
cd pocsag-golang
make build
# Binaries land in: bin/pocsag, bin/pocsag-decode, bin/pocsag-burst
```

---

## Encoder (`pocsag`)

Generate a POCSAG message as a WAV file.

**Required:**
- `-a` / `--address` — pager address (full 21-bit RIC/capcode, e.g. `1234567`)
- `-m` / `--message` — the message text
- `--type` — payload encoding: `numeric` or `alpha`

**Optional:**
- `-o` / `--output` — output WAV file (default: `output.wav`)
- `-f` / `--function` — 2-bit POCSAG function value to transmit: `0`, `1`, `2`, or `3` (default: `3`)
- `-b` / `--baud` — baud rate: `512`, `1200`, or `2400` (default: `1200`)
- `-e` / `--encrypt` — enable AES-256 encryption
- `-k` / `--key` — encryption password (required with `-e`)
- `-j` / `--json` — print result as JSON instead of human-readable text
- `-w` / `--waterfall` — save a waterfall spectrogram PNG of the signal

**Function bits vs payload encoding:**

| Setting | Goes over the air? | Purpose | Values | Example |
|---|---:|---|---|---|
| `-f` / `--function` | Yes | Sets the 2-bit function value in the POCSAG address codeword. Pagers use this as a programmed slot/alert selector. | `0`, `1`, `2`, `3` | `-f 1` |
| `--type` | No | Selects how this tool packs the following message codewords. This is an encoder instruction, not an extra POCSAG field. | `numeric`, `alpha` | `--type numeric` |

| Command intent | CLI flags | Result |
|---|---|---|
| Numeric | `-f 1 --type numeric` | Sends function bits `1`, encodes payload as numeric BCD. |
| Alpha | `-f 3 --type alpha` | Sends function bits `3`, encodes payload as 7-bit alphanumeric. |
| Alpha on another function slot | `-f 0 --type alpha` | Sends function bits `0`, encodes payload as 7-bit alphanumeric. |

**Examples:**

```bash
# Basic message
pocsag -a 123456 -m "HELLO WORLD" -f 3 --type alpha -o message.wav

# Numeric at 512 baud
pocsag -a 999888 -m "0123456789" -f 1 --type numeric -b 512 -o numeric.wav

# Numeric payload on function slot 1
pocsag -a 999888 -m "0123456789" -f 1 --type numeric -o numeric-f1.wav

# Fast 2400 baud
pocsag -a 123456 -m "FAST MSG" -f 3 --type alpha -b 2400 -o fast.wav

# With encryption
pocsag -a 123456 -m "SECRET" -f 3 --type alpha -e -k "mypassword" -o encrypted.wav

# With waterfall image
pocsag -a 123456 -m "HELLO WORLD" -f 3 --type alpha -o message.wav -w waterfall.png

# JSON output (great for scripts)
pocsag -a 123456 -m "TEST" -f 3 --type alpha -o test.wav --json
```

**Normal output:**
```
✅ Generated message.wav
   Address: 123456, Function: 3, Type: alphanumeric, Baud: 1200, Message: HELLO WORLD

Decode: pocsag-decode -i message.wav  or  multimon-ng -t wav -a POCSAG1200 message.wav
```

**JSON output:**
```json
{
  "success": true,
  "output": "message.wav",
  "address": 123456,
  "function": 3,
  "message": "HELLO WORLD",
  "baud": 1200,
  "encrypted": false,
  "type": "alphanumeric",
  "size": 38444
}
```

---

## Decoder (`pocsag-decode`)

Decode a POCSAG WAV back to text.

**Options:**
- `-i` / `--input` — input WAV file (required)
- `-b` / `--baud` — baud rate to try (default: `1200`)
- `-k` / `--key` — decryption password (if the message is encrypted)
- `-j` / `--json` — JSON output
- `-v` / `--version` — show version info

```bash
pocsag-decode -i message.wav
pocsag-decode -i message.wav -b 2400
pocsag-decode -i encrypted.wav -k "mypassword"
pocsag-decode -i message.wav --json
```

**Output:**
```
POCSAG1200: Decoded messages:
Address:  123456  Function: 3  ALPHA    Message: HELLO WORLD
```

---

## Burst Encoder (`pocsag-burst`)

Pack multiple messages for different pagers into a single WAV file.

**Options:**
- `-j` / `--json` — path to a JSON file listing the messages (required)
- `-o` / `--output` — output WAV file (default: `burst.wav`)
- `-b` / `--baud` — baud rate (default: `1200`)

**Input JSON format:**
```json
[
  {"address": 123456, "message": "FIRST MESSAGE", "function": 3, "payload_type": "alpha"},
  {"address": 789012, "message": "SECOND MESSAGE", "function": 3, "payload_type": "alpha"},
  {"address": 345678, "message": "0123456789", "function": 1, "payload_type": "numeric"}
]
```

```bash
pocsag-burst -j messages.json -o burst.wav
pocsag-burst -j messages.json -b 512 -o burst.wav
```

---

## Waterfall spectrogram

Pass `-w output.png` to the encoder and it generates a frequency×time spectrogram of the signal using an OpenGL 4.1 renderer. The image uses the PySDR colormap (dark blue → purple → red → yellow → white), which makes the FSK tones easy to spot even in a short transmission.

This runs headless — no window pops up, it just writes the PNG and exits.

```bash
pocsag -a 123456 -m "HELLO" -f 3 --type alpha -o msg.wav -w waterfall.png
```

There are two waterfall implementations in the library:
- `waterfall.go` — pure Go/CPU, no GPU required, generates PNGs programmatically
- `waterfall_gl.go` — OpenGL-accelerated, used by the CLI for better colour fidelity and real-time display

---

## Encryption

Messages can be encrypted with AES-256 before being packed into the POCSAG signal. The pipeline is:

```
Message → CRC32 checksum → AES-256 encrypt → Base64 encode → POCSAG packet
```

The encryption uses a password you supply, hashed with SHA-256 to produce the key. A random IV is generated per message to prevent replay attacks. CRC32 is checked on decode to catch any corruption or wrong-key attempts.

Encrypted messages appear as Base64 text in tools like `multimon-ng` that don't know about the encryption — they'll just show the raw ciphertext string.

```bash
# Encrypt
pocsag -a 123456 -m "CONFIDENTIAL" -f 3 --type alpha -e -k "strongpassword" -o enc.wav

# Decrypt
pocsag-decode -i enc.wav -k "strongpassword"
```

---

## Using as a Go library

Import as `github.com/sqpp/pocsag-golang/v2`.

**Encode and write a WAV:**
```go
import pocsag "github.com/sqpp/pocsag-golang/v2"

packet := pocsag.CreatePOCSAGPacketWithPayloadType(123456, "HELLO WORLD", 3, pocsag.PayloadTypeAlpha)
wavData := pocsag.ConvertToAudio(packet)
os.WriteFile("output.wav", wavData, 0644)
```

**Decode a WAV:**
```go
wavData, _ := os.ReadFile("message.wav")
messages, err := pocsag.DecodeFromAudio(wavData)
for _, msg := range messages {
    fmt.Println(msg.String())
}
```

**Key functions:**

| Function | Description |
|----------|-------------|
| `CreatePOCSAGPacketWithPayloadType(addr, msg, fn, type)` | Encode with function bits and payload encoding selected separately |
| `CreatePOCSAGPacketWithBaudRateAndPayloadType(addr, msg, fn, baud, type)` | Encode at a specific baud with explicit payload encoding |
| `ConvertToAudio(data)` | Convert to WAV bytes (1200 baud) |
| `ConvertToAudioWithBaudRate(data, baud)` | Convert to WAV at specific baud |
| `DecodeFromAudio(wavData)` | Decode a WAV (assumes 1200 baud) |
| `DecodeFromAudioWithBaudRate(wavData, baud)` | Decode at specific baud |
| `DecodeFromBinary(data)` | Decode raw POCSAG bytes |
| `DecodeFromBinaryWithPayloadType(data, type)` | Decode raw POCSAG bytes with explicit numeric/alpha interpretation |

---

## Testing

```bash
go test -v ./...
```

The integration test suite covers every combination of baud rate (512, 1200, 2400) and message type (alpha, numeric, encrypted), including 500-character long messages, burst transmissions, and cross-validation against `multimon-ng`.

> If you're on Windows and want the `multimon-ng` cross-check to run, have it available in WSL.

---

## About addresses

POCSAG addresses are full 21-bit RIC/capcodes. The encoder places each message codeword in the correct frame within the burst (`address % 8`), which is required by ITU-R M.584-2.

Tools like `multimon-ng` typically display `(address / 8) * 8`, so an address of `1234567` will show up as `1234560` in their output — that's expected.

---

## Credits

Encoder originally ported from [pocsag-tool](https://github.com/hazardousfirmware/pocsag-tool) by hazardousfirmware.

## License

BSD-2-Clause
