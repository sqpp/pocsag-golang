# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [2.2.0]

### Fixed
- **POCSAG address (RIC/capcode) and frame placement** – Address handling now matches ITU-R M.584-2:
  - Address is the full 21-bit RIC/capcode (pager identity); no longer normalized to a multiple of 8.
  - Address codeword is placed in the correct frame (`address % 8`) within each batch (8 frames × 2 codewords). Previously all messages were effectively sent in frame 0, so pagers with addresses like 1234561–1234567 would not see their pages.
  - `EncodeAddress` uses the full address (18 MSBs in the codeword); frame placement is done in `CreatePOCSAGBurstWithBaudRate`.
- **Makefile LDFLAGS** – Version/build-time variables were not applied because the package path was `github.com/sqpp/pocsag-golang` instead of `github.com/sqpp/pocsag-golang/v2`. LDFLAGS now use the correct `/v2` path so `pocsag --version` shows real Build Time, Git Commit, and Build Architecture.

### Changed
- **CLI** – Encoder no longer normalizes the address; the value you pass is the full RIC and is used as-is. Decoders (e.g. multimon-ng, pocsag-decode) typically display `(address/8)×8`. Default WAV output works with both pocsag-decode and multimon-ng.
- **README** – Updated encoder/decoder/burst inputs and outputs, address/RIC description, burst example, installation paths (`/v2`), and testing section (default output works with both decoders).

---

## [2.1.1] - 2025-12-16

### Changed
- Fix release workflow to avoid concurrent git operations (`7504b46`).
- Fix Go version in GitHub Actions workflow (`28ea3e6`).

### Added
- FSK waterfall support (`3b2fd8c`).
- Waterfall restructure (`59c7dd0`).

### Fixed
- Audio fixes (`2067ea8`).
- WAV file headers for Firefox compatibility (`7338442`).
- go.mod fixes (`597e14f`, `3f4bf6a`).

---

## [2.1.0] - 2025-10-26

### Added
- Comprehensive build-time version information (`f80b01c`).
- Binary architecture details in version info (`c2ae229`).
- Dynamic version information with author and project URL (`3562510`).

### Fixed
- Makefile (`f743113`).

### Changed
- Version set to 2.1.0 (`6bc7361`).
- README update for 2.0.0 (`4bf6e3c`).

---

## [2.0.0] - 2025-10-26

### Added
- POCSAG encryption support (AES-256/AES-128) (`777a132`).
- Support for POCSAG 512 and 2400 baud (`396120a`).
- Automatic release workflow (`2bf3fa7`).
- GitHub Actions: build each command separately (`aa7c74b`), Go version 1.22 (`e3bb050`).
- Note about POCSAG address format (multiples of 8) (`6e9d479`).
- Burst mode: multiple messages in one transmission (`2ff11cd`).
- Burst mode documentation in README (`5fdc5a8`).
- GitHub Actions go.yml (`7011e8c`).
- JSON output support for API integration (`4795b78`).
- JSON output documentation and examples in README (`5106879`, `7fefae2`).

### Fixed
- GitHub Actions cache issues for projects without dependencies (`ec8947a`).
- go.yml: setup-go@v5 with 1.22.x (`5034e65`).
- Long message truncation bug (`91d3de5`).
- Numeric message termination (`de625ba`).

### Removed
- pocsag-burst (temporary removal in `2e8cda3`; later re-added).

---

## [1.5.0] and earlier

- Command-line tools (`8984b22`).
- Module path fix (marcell → sqpp) (`32ce594`).
- Initial commit: POCSAG encoder and decoder (`f72e0a6`).

For full git history run: `git log --oneline`.

[2.2.0]: https://github.com/sqpp/pocsag-golang/compare/v2.1.1...HEAD
[2.1.1]: https://github.com/sqpp/pocsag-golang/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/sqpp/pocsag-golang/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/sqpp/pocsag-golang/compare/v1.5.0...v2.0.0
