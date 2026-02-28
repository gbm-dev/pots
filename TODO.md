# TODO

## D-Modem Implementation Track

- [ ] Validate `dmodem` call success rate across at least 20 dial attempts per site profile.
- [ ] Tune default modem init commands for reliability (`ATX3`, modulation caps via `AT+MS`).
- [ ] Add per-call media quality capture (loss/jitter/RTT) and correlate against `NO CARRIER`.
- [ ] Add an operational runbook for restarting/recovering individual modem instances.

## Fallback: Option 3 Custom R&D (if D-Modem does not meet reliability targets)

- [ ] Prototype direct data-mode support path by extending/replacing the current softmodem stack.
- [ ] Evaluate whether `iaxmodem` + `spandsp` can be adapted for full-duplex data mode (not fax-only flows).
- [ ] Build a reproducible modem test harness (known-good dialup targets + scripted AT command matrix).
- [ ] Define go/no-go criteria for production viability:
  - Minimum successful connect rate under packet-loss/jitter thresholds.
  - Stable disconnect handling and deterministic `NO CARRIER`/`CONNECT` reporting.
  - Multi-channel concurrency without cross-channel interference.
- [ ] If adaptation is not viable, document a replacement architecture and migration plan.
