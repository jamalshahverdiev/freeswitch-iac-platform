# HOWTO: custom IVR prompt (flite + sox) end to end

A precise, reproducible runbook for the workflow we used: generate a prompt
`.wav` from text on the dev box, ship it to the FreeSWITCH server, and play it
from an IVR managed by Terraform. This is "Goal A" (pre-recorded prompt) — for
runtime text-to-speech see `docs/ivr-audio.md` (the `speak` action).

## Topology recap (who does what)

| Box | Role | Tools |
|---|---|---|
| Dev box (WSL2 Debian, `172.31.30.216`) | generate + version the `.wav`, run Terraform | `flite`, `sox`, `tofu` |
| FreeSWITCH server (`192.168.48.143`, ssh `root`) | stores the prompt file, plays it | FreeSWITCH |

The control-plane container **cannot** write to the FS server filesystem, so
shipping the file is a manual ops step (`scp`). Terraform only references it.

---

## Step 1 — install the audio tools (dev box, once)

`flite` is an offline TTS engine; `sox` converts/resamples audio.

```bash
sudo apt-get update && sudo apt-get install -y flite sox
```

Verify:

```bash
command -v flite sox          # /usr/bin/flite  /usr/bin/sox
```

flite voices: `slt` (female), `rms` / `awb` / `kal` (male).

---

## Step 2 — generate the prompt from text (dev box)

Keep the source under version control in the repo:

```bash
cd freeswitch-iac-platform
mkdir -p deploy/freeswitch/sounds

TXT="This prompt was pre rendered with flite and shipped to the FreeSWITCH server, then played by Terraform."
flite -voice slt -t "$TXT" deploy/freeswitch/sounds/tf-custom-prompt.src.wav
```

Inspect what flite produced (its native rate is not 8 kHz):

```bash
soxi deploy/freeswitch/sounds/tf-custom-prompt.src.wav
```

---

## Step 3 — convert to FreeSWITCH narrowband (8 kHz mono 16-bit)

FreeSWITCH picks the sample-rate subfolder matching the call; `8000` (G.711) is
the safe baseline for SIP/PSTN. Resample to **8 kHz, 1 channel, 16-bit PCM**:

```bash
sox deploy/freeswitch/sounds/tf-custom-prompt.src.wav \
    -r 8000 -c 1 -b 16 \
    deploy/freeswitch/sounds/tf-custom-prompt.wav

soxi deploy/freeswitch/sounds/tf-custom-prompt.wav
#   Channels       : 1
#   Sample Rate    : 8000
#   Precision      : 16-bit
```

> Optional: ship higher rates too (16000/32000/48000) with the **same filename**
> in their folders, and FreeSWITCH will choose the best for each call. For a
> demo, 8000 alone is enough.

Both files now live in `deploy/freeswitch/sounds/` (tracked in git).

---

## Step 4 — ship the file to the FreeSWITCH server

Target path convention:
`/usr/share/freeswitch/sounds/<lang>/<region>/<voice>/ivr/<rate>/<name>.wav`.
On this server that is the `en/us/callie/ivr/8000/` folder.

```bash
DST=/usr/share/freeswitch/sounds/en/us/callie/ivr/8000/tf-custom-prompt.wav

# copy (sshpass used here for the lab password; prefer keys in real setups)
sshpass -p "$FS_SSH_PASS" scp -o StrictHostKeyChecking=no \
  deploy/freeswitch/sounds/tf-custom-prompt.wav root@192.168.48.143:$DST

# make it owned by the freeswitch user
sshpass -p "$FS_SSH_PASS" ssh -o StrictHostKeyChecking=no root@192.168.48.143 \
  "chown freeswitch:freeswitch $DST && ls -la $DST"
```

No `reloadxml` is needed for *sound files* — only the dialplan that references
them must be loaded (next steps). For repeatable delivery use Ansible / a CI
`scp` / a sync agent instead of a one-off copy.

---

## Step 5 — reference the file from Terraform

In a `freeswitch_dialplan_extension`, reference the file **relative to the
voice's root** as `ivr/<name>.wav` (FreeSWITCH expands lang/region/voice/rate).
See `examples/ivr-prompt/main.tf`:

```hcl
resource "freeswitch_dialplan_extension" "prompt" {
  name     = "custom-prompt"
  domain   = "192.168.48.143"   # or data.freeswitch_domain.main.name
  context  = "company"
  priority = 92

  condition {
    field      = "destination_number"
    expression = "^(9200)$"
    action { application = "answer" }
    action {
      application = "playback"
      data        = "ivr/tf-custom-prompt.wav"
    }
  }
}

# Re-read the dialplan in FreeSWITCH after changes.
resource "freeswitch_reloadxml" "apply" {
  triggers = { ext = freeswitch_dialplan_extension.prompt.id }
}
```

As a menu prompt with digit collection (6th arg is the prompt file):

```hcl
action {
  application = "play_and_get_digits"
  data = "1 1 3 5000 # ivr/tf-custom-prompt.wav ivr/ivr-that_was_an_invalid_entry.wav choice \\d"
}
```

---

## Step 6 — apply

```bash
export TF_CLI_CONFIG_FILE=$HOME/.terraformrc   # dev_overrides for the local provider
tofu -chdir=examples/ivr-prompt apply
```

The `freeswitch_reloadxml` resource runs `reloadxml` via ESL so FreeSWITCH
picks up the new extension.

---

## Step 7 — verify it plays

No softphone needed — originate a loopback call into the extension and check the
log:

```bash
sshpass -p "$FS_SSH_PASS" ssh -o StrictHostKeyChecking=no root@192.168.48.143 '
  fs_cli -x "originate loopback/9200/company &park()" >/dev/null 2>&1 &
  sleep 5; fs_cli -x "hupall NORMAL_CLEARING" >/dev/null 2>&1
  grep -i "playing file.*tf-custom-prompt" /var/log/freeswitch/freeswitch.log | tail'
```

Expected (what we observed):

```
EXECUTE loopback/9200-b playback(ivr/tf-custom-prompt.wav)
done playing file /usr/share/freeswitch/sounds/en/us/callie/ivr/tf-custom-prompt.wav
```

A real SIP phone (registered as a user in domain `192.168.48.143`, e.g. `2001`)
dialing `9200` hears the prompt.

---

## Notes & gotchas

- **Format matters.** FreeSWITCH wants PCM `.wav`; mono 8 kHz 16-bit is the safe
  baseline. Wrong rate/codec → silence or errors. Verify with `soxi`.
- **Relative path.** Use `ivr/<name>.wav` in the dialplan, NOT the absolute path.
  The leading folders (`en/us/callie`) and the rate folder are chosen by FS.
- **Filename = identity.** Re-shipping the same filename replaces the prompt;
  the dialplan doesn't change, so only `reloadxml` (or nothing, for a plain file
  swap) is needed.
- **Ops boundary.** Generating + shipping the file is outside Terraform (the
  control-plane can't reach the FS filesystem). Keep `.wav` sources in
  `deploy/freeswitch/sounds/` and automate delivery (Ansible/CI/agent).
- **Voices/quality.** flite is robotic. For production-grade audio, record real
  audio or use a better TTS, then follow Steps 3–7 unchanged.

## Cleanup (demo)

```bash
tofu -chdir=examples/ivr-prompt destroy            # removes extension 9200
ssh root@192.168.48.143 'rm -f /usr/share/freeswitch/sounds/en/us/callie/ivr/8000/tf-custom-prompt.wav'
```
