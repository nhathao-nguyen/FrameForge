# Acceptance evidence

This directory contains sanitized, reviewable acceptance outputs. Evidence files must not contain
passwords, bearer tokens, signing keys, presigned URLs, user media or other private payloads.

- `t550-lan-client-pass-20260818.json` is the physical second-device T550 LAN acceptance result from
  Windows client `DESKTOP-92ICS6C` (`192.168.1.29`) against server `192.168.1.18`. The source JSON was
  produced by `tools/accept-lan-client.ps1`; credentials were entered interactively and are absent.
