#!/usr/bin/env bash
# Runs ON the FreeSWITCH server. Enables mod_xml_curl and merges the
# control-plane ESL ACL into acl.conf.xml (idempotent).
set -euo pipefail
CFG=/etc/freeswitch/autoload_configs

# 1) Enable mod_xml_curl (uncomment the load line if commented).
if grep -q '<!-- *<load module="mod_xml_curl"/> *-->' "$CFG/modules.conf.xml"; then
  sed -i 's#<!-- *<load module="mod_xml_curl"/> *-->#<load module="mod_xml_curl"/>#' "$CFG/modules.conf.xml"
  echo "enabled mod_xml_curl in modules.conf.xml"
elif grep -q '<load module="mod_xml_curl"/>' "$CFG/modules.conf.xml"; then
  echo "mod_xml_curl already enabled"
else
  # not present at all: add it after mod_xml_cdr or before </modules>
  sed -i 's#</modules>#    <load module="mod_xml_curl"/>\n</modules>#' "$CFG/modules.conf.xml"
  echo "added mod_xml_curl to modules.conf.xml"
fi

# 2) Merge the control-plane ACL list (idempotent).
if grep -q 'name="control-plane"' "$CFG/acl.conf.xml"; then
  echo "control-plane ACL already present"
else
  python3 - "$CFG/acl.conf.xml" <<'PY'
import sys
path = sys.argv[1]
block = """    <list name="control-plane" default="deny">
      <node type="allow" cidr="127.0.0.1/32"/>
      <node type="allow" cidr="::1/128"/>
      <node type="allow" cidr="172.31.30.216/32"/>
    </list>
"""
data = open(path).read()
data = data.replace("</network-lists>", block + "  </network-lists>", 1)
open(path, "w").write(data)
print("inserted control-plane ACL list")
PY
fi
