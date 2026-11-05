"""Returns both agents to an empty state before a rehearsal, through OpenClaw's
own commands: each session is forgotten by memory search, because a deleted
session's archive stays searchable, then deleted; then the host agent's notes
go. An agent's main session stays: OpenClaw refuses to delete it, and no
WhatsApp conversation runs in it. Runs inside the openclaw container
(`make agents-reset`)."""
import json
import pathlib
import shutil
import subprocess

MEMORY_DIR = pathlib.Path("/opt/demo/workspace-host/memory")


def openclaw(*args: str) -> dict:
    done = subprocess.run(["node", "openclaw.mjs", *args, "--json"], stdout=subprocess.PIPE, text=True)
    if done.returncode != 0:
        raise SystemExit(f"openclaw {' '.join(args)} failed: {done.stdout}")
    return json.loads(done.stdout)


deleted = 0
for session in openclaw("sessions", "--all-agents", "--limit", "all")["sessions"]:
    key, agent = session["key"], session["agentId"]
    openclaw("memory", "forget", "--agent", agent, "--session", key)
    if key != f"agent:{agent}:main":
        openclaw("sessions", "delete", key, "--agent", agent, "--yes")
        deleted += 1

notes = 0
for entry in MEMORY_DIR.iterdir():
    notes += 1
    if entry.is_dir():
        shutil.rmtree(entry)
    else:
        entry.unlink()

print(f"Deleted {deleted} conversations and {notes} entries of the host agent's notes.")
