#!/usr/bin/env python3
"""Links the business number to OpenClaw's WhatsApp account (ACCOUNT, `default`
unless set): prints the login QR in the terminal and waits until the business
phone scans it. Runs inside the openclaw container (`make whatsapp-link`).
`openclaw channels login` cannot run here: it writes openclaw.json, which is
read-only; the Gateway's web.login.start and web.login.wait write nothing."""
import base64, json, os, struct, subprocess, sys, zlib

ACCOUNT = os.environ.get("ACCOUNT", "default")
WAIT_MS = 120000


def gateway(method, params, timeout_ms):
    done = subprocess.run(
        ["node", "/app/openclaw.mjs", "gateway", "call", method, "--params", json.dumps(params), "--json",
         "--timeout", str(timeout_ms), "--token", os.environ["OPENCLAW_GATEWAY_TOKEN"]],
        capture_output=True, text=True, cwd="/app",
    )
    # The command line carries the gateway token: report the gateway's own words only.
    if done.returncode != 0:
        sys.exit(f"ERROR: gateway call {method} failed: {(done.stderr or done.stdout).strip()[-800:]}")
    out = done.stdout
    return json.loads(out[out.index("{"):])


def png_pixels(data):
    """Decodes a non-interlaced 8-bit PNG into rows of luminance values."""
    pos, idat = 8, b""
    while pos < len(data):
        length, kind = struct.unpack(">I4s", data[pos:pos + 8])
        body = data[pos + 8:pos + 8 + length]
        if kind == b"IHDR":
            width, height, depth, color, _, _, interlace = struct.unpack(">IIBBBBB", body)
        elif kind == b"IDAT":
            idat += body
        pos += 12 + length
    channels = {0: 1, 2: 3, 4: 2, 6: 4}.get(color)
    if depth != 8 or interlace or channels is None:
        sys.exit(f"ERROR: unsupported QR image (depth {depth}, color {color}, interlace {interlace})")
    raw, stride, rows, prev = zlib.decompress(idat), width * channels, [], bytearray(width * channels)
    for y in range(height):
        start = y * (stride + 1)
        kind, line = raw[start], bytearray(raw[start + 1:start + 1 + stride])
        for i in range(stride):
            left = line[i - channels] if i >= channels else 0
            up, upleft = prev[i], prev[i - channels] if i >= channels else 0
            if kind == 1:
                line[i] = (line[i] + left) & 255
            elif kind == 2:
                line[i] = (line[i] + up) & 255
            elif kind == 3:
                line[i] = (line[i] + (left + up) // 2) & 255
            elif kind == 4:
                p = left + up - upleft
                pa, pb, pc = abs(p - left), abs(p - up), abs(p - upleft)
                line[i] = (line[i] + (left if pa <= pb and pa <= pc else up if pb <= pc else upleft)) & 255
        rows.append([line[x * channels] for x in range(width)])
        prev = line
    return rows


def print_qr(data_url):
    png = base64.b64decode(data_url.split(",", 1)[1])
    rows = png_pixels(png)
    dark = [[value < 128 for value in row] for row in rows]
    top = next(y for y, row in enumerate(dark) if any(row))
    left = dark[top].index(True)
    finder = 0
    while dark[top][left + finder]:
        finder += 1
    module = finder / 7
    count = round((len(dark[top]) - 2 * left) / module)
    grid = [[dark[int(top + (r + 0.5) * module)][int(left + (c + 0.5) * module)] for c in range(count)] for r in range(count)]
    quiet = [[False] * (count + 8)] * 4
    grid = quiet + [[False] * 4 + row + [False] * 4 for row in grid] + quiet + [[False] * (count + 8)]
    # Light modules are drawn, dark ones left to the terminal's background: scan it from a dark terminal.
    glyph = {(True, True): "█", (True, False): "▀", (False, True): "▄", (False, False): " "}
    for r in range(0, len(grid) - 1, 2):
        print("".join(glyph[(not a, not b)] for a, b in zip(grid[r], grid[r + 1])))
    print("Scan it from the business phone: WhatsApp Business > Settings > Linked devices > Link a device.", flush=True)


started = gateway("web.login.start", {"channel": "whatsapp", "accountId": ACCOUNT}, 30000)
if not started.get("qrDataUrl"):
    sys.exit(started.get("message", "WhatsApp login did not start"))
current = started["qrDataUrl"]
print_qr(current)
while True:
    params = {"channel": "whatsapp", "accountId": ACCOUNT, "timeoutMs": WAIT_MS, "currentQrDataUrl": current}
    if started.get("sessionKey"):
        params["sessionKey"] = started["sessionKey"]
    result = gateway("web.login.wait", params, WAIT_MS + 10000)
    if result.get("connected"):
        print(result.get("message", "WhatsApp is linked."))
        break
    if result.get("qrDataUrl") and result["qrDataUrl"] != current:
        current = result["qrDataUrl"]
        print("\nThe QR changed:")
        print_qr(current)
        continue
    sys.exit(result.get("message", "WhatsApp login ended without a link"))
