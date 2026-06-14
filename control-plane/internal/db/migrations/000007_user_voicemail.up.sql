-- Typed voicemail mailbox per user. NULL = no voicemail configured (nothing
-- rendered into the directory); a JSON object renders vm-* directory params.
-- Shape: {"enabled":bool,"password":str,"email":str,"attach_file":bool,"email_all":bool}
ALTER TABLE users ADD COLUMN IF NOT EXISTS voicemail JSONB;
