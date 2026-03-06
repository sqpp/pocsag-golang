# Changelog

All notable changes are documented here.
Format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [2.3.0] - 2026-03-06

### OpenGL Waterfall (GPU-accelerated spectrogram)

The waterfall visualisation got a serious upgrade. Instead of drawing pixels in software like before, it now uses OpenGL 4.1 to render the spectrogram directly on the GPU — which means it's fast, smooth, and can handle real-time scrolling without breaking a sweat.


- **GPU-powered rendering** via OpenGL 4.1. Each FFT row is uploaded as a floating-point texture and coloured on the GPU using the PySDR colormap shader — the same gradient you'd see in SDR++ or GQRX (dark blue → purple → red → yellow → white).
- **Headless PNG export** — pass `-w waterfall.png` to the CLI and it creates a full spectrogram image without ever showing a window. Useful for scripts or CI pipelines.
- **Calibrated colormap** — the dB-to-colour mapping is now calculated from actual measured FFT power levels of synthetic POCSAG signals (rather than guessing), so the background sits in dark blue and signal peaks hit yellow/white as expected.

---

## [2.2.2] - 2026-03-05

### Solid integration tests and a much more reliable decoder

- Wrote an exhaustive integration test suite (`integration_test.go`) that tests every combination of baud rate and message type — including 500-character messages — against both the internal decoder and `multimon-ng`.
- The decoder now scans bit-by-bit for the POCSAG sync word instead of assuming byte alignment. This killed a whole class of sync failures that happened when a byte boundary happened to fall in the middle of the preamble.
- Fixed subtle timing drift in both the encoder and decoder by switching to cumulative `math.Round` logic instead of truncating sample indices per-bit. At 2400 baud over long messages this was causing real problems.
- Added BCH(31,21) and parity validation to the decode loop so corrupted noise can't sneak through as a valid message anymore.
- Fixed a burst encoder bug where encoding multiple messages at once would cause them to overwrite each other's state.
- Fixed an integer truncation bug in `DecodeFromLiveStreamWithDecryption` that caused gradual drift on long recordings.

---

## [2.2.1]

### DPLL clock recovery and end-to-end decryption

- Added a Phase-Locked Loop (DPLL) to the decoder for real-world clock recovery. It watches zero-crossings in the demodulated signal and nudges the sampling phase to stay in sync — makes a big difference with recordings that have any timing wander.
- The decoder can now automatically detect and handle a DC offset or DC drift on the baseband signal, so you don't need to pre-process your recordings.
- `pocsag-decode` now supports AES-256 decryption end-to-end. Pass `-k yourpassword` and it decrypts on the fly.

**Also fixed a pretty embarrassing decoder bug:** POCSAG bursts with more than one 16-codeword batch were being silently truncated after the first batch. If you were decoding longer transmissions and only seeing part of the message, this was why. Fixed by properly counting codewords per batch and continuing through intermediate sync words.

Other fixes: Base64 padding repair for encrypted messages, correct address reconstruction from batch frame indices, and dynamic WAV header parsing so files with non-standard offsets now work too.

---

## [2.2.0] - 2026-02-20

### Correct POCSAG addressing (finally matches the spec)

Turned out the address handling wasn't quite right. POCSAG uses a full 21-bit RIC/capcode, and the message codeword has to go into a specific frame within the burst (`address % 8`). We were putting everything in frame 0, which meant pagers with addresses like 1234561–1234567 wouldn't receive anything. Fixed.

Also fixed the Makefile LDFLAGS so `pocsag --version` actually shows real build info (commit hash, timestamp, arch) instead of placeholder strings.

---

## [2.1.1] - 2025-12-16

- Fixed the GitHub Actions release workflow (concurrent git ops were causing flaky builds).
- Added initial FSK waterfall visualisation support.
- Fixed WAV headers for Firefox compatibility.

---

## [2.1.0] - 2025-10-26

- Added proper build-time version info: binary now knows its own version, git commit, build time, and architecture.

---

## [2.0.0] - 2025-10-26

Big feature release:

- **AES-256/AES-128 encryption** for secure messages.
- **Multi-baud support** — 512 and 2400 baud added alongside 1200.
- **Burst mode** — encode multiple messages for different pagers into one WAV.
- **JSON output** for easy API and script integration.
- Automatic GitHub Actions release workflow.

---

## [1.5.0] and earlier

- Command-line tools, initial encoder/decoder, module path fix.
- See `git log --oneline` for the full history.

---

[2.3.0]: https://github.com/sqpp/pocsag-golang/compare/v2.2.2...HEAD
[2.2.2]: https://github.com/sqpp/pocsag-golang/compare/v2.2.1...v2.2.2
[2.2.1]: https://github.com/sqpp/pocsag-golang/compare/v2.2.0...v2.2.1
[2.2.0]: https://github.com/sqpp/pocsag-golang/compare/v2.1.1...v2.2.0
[2.1.1]: https://github.com/sqpp/pocsag-golang/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/sqpp/pocsag-golang/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/sqpp/pocsag-golang/compare/v1.5.0...v2.0.0
