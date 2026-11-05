#!/bin/sh
# Replaces every older copy of each package below, in the OpenClaw base image and in the plugins installed into it,
# with the newest release of its major, checked against the registry's integrity. Each line is a fix newer than the
# OpenClaw base image.
set -eu
{

# package major release integrity
fixes='
undici 6 6.29.1 sha512-EvRlUt/QqAEvXdQeRMYUfwOQsfzpDztykrG2owGEE/rBOBoYv9uF39ZzsNDh4lTFHhTr2XOZjKkYgueD12ZmGA==
undici 8 8.12.3 sha512-+fdndUTwBP86cVjihTp21hZHo7d6oj1c/47XPA3lL8mM0XHs+C4EkcjmgRsFnFk2f/UONGDy9ItBItmXsLOQHw==
body-parser 2 2.4.0 sha512-Ifk1KcLrxaGt1occQagQNFNNOqRt1LFuehEnHHzB8Z6cRgIxRU1TBaiIsUiCdA+gy5PL7Y1IX31DNKkRmxtXjQ==
brace-expansion 5 5.0.12 sha512-YovQ3rzhaLMIrDjNDMkNS01tea93qhEhG5xy8f6+R0l+dw3Ki+5sCoIoI942iuLZTHWogWktgwVDhU09iNEimQ==
dompurify 3 3.4.17 sha512-B/pSlYVHQ58kqISD/yDjuJ1yfzI34m1gF4lAod2bwSRhiN4K5Cl3H8Af7hqB2PmTL911tdEudnnXn5FnhRoz0w==
markdown-it 15 15.0.3 sha512-+Az7Ojz3iRGu8KYMvJ6xVhvrxP3eE+vEtJX75Ow88XBpcQNV/0lGX5ZynIIBftREo4DC2jsveAMDaeltmm8QvQ==
zod 4 4.6.7 sha512-g8W2D2kU6a8untc053fX1uVBKzM7yqAxW/3zwk83+6RyF2+SKOuV+c0wBDhUwzV1PaGapmZH+rmcUzPf+3CGwg==
joi 18 18.3.1 sha512-mV9M4Llh4va3NT90p7KBtQhoZqjs8xLX7EWYE4y/GG3yLF6REbIBEHSUb0UX1JXRht2w5tiLSsvmpYc5akiqIw==
'

work=$(mktemp -d)
cd "$work"
echo "$fixes" > fixes
while read -r name major release integrity <&3; do
	[ -n "$name" ] || continue
	npm pack --silent "$name@$release" >/dev/null
	archive="${name}-${release}.tgz"
	have=$(node -e 'process.stdout.write("sha512-" + require("crypto").createHash("sha512").update(require("fs").readFileSync(process.argv[1])).digest("base64"))' "$archive")
	[ "$have" = "$integrity" ] || { echo "$archive: integrity $have, expected $integrity" >&2; exit 1; }

	replaced=0
	find / -xdev -path "*/node_modules/$name/package.json" -type f > manifests
	while read -r manifest; do
		version=$(node -p "require('$manifest').version")
		[ "${version%%.*}" = "$major" ] || continue
		older=$(node -p "const [a, b] = ['$version', '$release'].map(v => v.split('.').map(Number)); a[0] - b[0] || a[1] - b[1] || a[2] - b[2]")
		[ "$older" -lt 0 ] || continue
		dir=$(dirname "$manifest")
		find "$dir" -mindepth 1 -delete
		tar -xzf "$archive" --strip-components=1 -C "$dir"
		echo "$name $version -> $release: $dir"
		replaced=$((replaced + 1))
	done < manifests
	[ "$replaced" -gt 0 ] || { echo "no bundled $name $major.x replaced: the base image changed; drop its line if it no longer needs it" >&2; exit 1; }
done 3< fixes
cd /
rm -rf "$work"
exit 0
}
