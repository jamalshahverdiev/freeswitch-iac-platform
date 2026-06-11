# IVR audio: prompts & text-to-speech

There are two ways to give an IVR a voice, both driven from Terraform
(`freeswitch_dialplan_extension` actions):

1. **Runtime TTS** — you put the *text* in Terraform and FreeSWITCH synthesizes
   speech on the fly (`mod_flite`). No files. Best for dynamic/quick prompts.
2. **Pre-recorded prompt files** — you generate/record a `.wav`, drop it under
   the FreeSWITCH sounds tree, and reference it with `playback`. Best for
   high-quality / branded / multi-language prompts.

## 1. Runtime TTS (text in Terraform)

Two engines are enabled on the server:

| Engine (speak prefix) | Quality | Voices |
|---|---|---|
| `tts_commandline` → **Piper** (neural) ⭐ | natural, human-like | `en_US-ryan-medium` (male), add more from huggingface `rhasspy/piper-voices` into `/opt/piper/voices/` (e.g. `en_US-amy-medium` female) |
| `flite` | robotic (fallback) | `slt` (female), `rms`/`awb`/`kal` (male) |

Piper lives at `/opt/piper/` and is wired through `mod_tts_commandline`
(`deploy/freeswitch/tts_commandline.conf.xml`; the module single-quotes
`${text}` itself — don't add quotes; synthesis takes ~1-4 s per prompt, so
pre-render long static prompts for production). Usage is identical, just a
different engine prefix:

```hcl
action {
  application = "speak"
  data        = "tts_commandline|en_US-ryan-medium|Welcome. Press one for sales."
}
```

```hcl
resource "freeswitch_dialplan_extension" "tts_menu" {
  name     = "tts-menu"
  domain   = "192.168.48.143"
  context  = "company"
  priority = 90

  condition {
    field      = "destination_number"
    expression = "^(9100)$"
    action { application = "answer" }
    action {
      application = "speak"
      data        = "flite|slt|Welcome. Press one for sales, or two for support."
    }
  }
}
```

Working example: `examples/ivr-tts/` (the prompt text is a Terraform variable).
Dial `9100` and the text is spoken. To collect a digit after a TTS prompt, use a
separate `speak` then route, or pre-render the prompt to a file (below) and use
it in `play_and_get_digits`.

## 2. Pre-recorded prompt files

### 2.1 Where the files live

```
/usr/share/freeswitch/sounds/<lang>/<region>/<voice>/ivr/<rate>/<name>.wav
```

On this server:

```
/usr/share/freeswitch/sounds/en/us/callie/ivr/8000/<name>.wav    # 8 kHz (narrowband / G.711)
                                              /16000/            # 16 kHz (wideband)
                                              /32000/  /48000/   # higher rates
```

- Only the **`callie`** voice (female) prompt pack is installed.
- FreeSWITCH auto-picks the sample-rate subfolder matching the call; for SIP/PSTN
  `8000` is the common one. Provide at least the `8000/` file. You can ship
  multiple rates with the same filename for best quality.
- In the dialplan you reference it **relative to the voice/ivr root**, i.e.
  `ivr/<name>.wav` (FreeSWITCH expands lang/region/voice/rate automatically).

### 2.2 Generate or record the `.wav`

Any source works (a recording, a TTS service, etc.). Offline with the `flite`
CLI (install once: `apt-get install -y flite`):

```bash
flite -voice slt -t "Your custom prompt text" prompt.wav    # 8 kHz mono PCM by default
```

Make sure the result is mono PCM (`.wav`). To match a specific rate:

```bash
flite -voice slt -t "..." -o prompt.wav        # then, if needed:
sox prompt.wav -r 8000 -c 1 prompt8k.wav        # resample to 8 kHz mono
```

### 2.3 Put it on the FreeSWITCH server

The control-plane runs in a container on the dev box and **cannot** write to the
FreeSWITCH server's filesystem, so shipping prompt files is an **ops step**, not
something Terraform does. Options:

```bash
# one-off copy
scp prompt.wav root@192.168.48.143:/usr/share/freeswitch/sounds/en/us/callie/ivr/8000/

# fix ownership/perms if needed
ssh root@192.168.48.143 'chown freeswitch:freeswitch \
  /usr/share/freeswitch/sounds/en/us/callie/ivr/8000/prompt.wav'
```

For repeatable delivery, use config management (Ansible/scp in CI) or a small
sync agent on the FS server. Keep your source `.wav` files in the repo (e.g.
`deploy/freeswitch/sounds/`) so they're version-controlled.

### 2.4 Reference it from Terraform

```hcl
resource "freeswitch_dialplan_extension" "welcome" {
  name     = "welcome"
  domain   = "192.168.48.143"
  context  = "company"
  priority = 91

  condition {
    field      = "destination_number"
    expression = "^(9200)$"
    action { application = "answer" }
    action {
      application = "playback"
      data        = "ivr/prompt.wav"   # -> en/us/callie/ivr/<rate>/prompt.wav
    }
  }
}
```

As an IVR menu prompt with digit collection (the 6th arg is the prompt file):

```hcl
action {
  application = "play_and_get_digits"
  # min max tries timeout terminators  prompt            invalid           var       regexp
  data = "1 1 3 5000 # ivr/prompt.wav ivr/ivr-that_was_an_invalid_entry.wav choice \\d"
}
```

## Which to use

| | Runtime TTS (`speak`) | Pre-recorded (`playback`) |
|---|---|---|
| Source of truth | text in Terraform | `.wav` file (shipped out-of-band) |
| Quality | robotic (flite) | as good as the recording |
| Setup | none (mod_flite loaded) | generate + scp the file |
| Change a prompt | edit HCL, `apply` | re-record/ship the file |

Apply changes with the `freeswitch_reloadxml` resource (or
`POST /api/v1/runtime/reloadxml`) so FreeSWITCH re-reads the dialplan.
