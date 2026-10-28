#!/bin/sh
# Records the demo video of rexamine in the Rex app: an agent finishes a change,
# rexamine opens beside it, a review comment goes to the agent, and the agent's
# fix shows up in the diff.
#
#   scripts/demo/demo.sh [output directory]
#
# It writes demo.gif to the output directory, the assets directory of the
# repository by default. It needs:
#   - Rex with Remote Control on, in Settings under Rex Server
#   - rexamine set up in Rex, with `rexamine setup`
#   - permission for the terminal to record the screen
#   - ffmpeg, jq, git, and python3
#
# The Rex window must fill the screen, because the video crops a fixed part of
# it. Set DEMO_CROP to w:h:x:y in screen pixels to crop another part.
#
# While it records, the Rex app shows the demo for about 20 seconds, and then
# goes back to the session that the script runs in. Do not type in that time,
# because the keys go to the demo.
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
out_dir="${1:-$(dirname "$(dirname "$here")")/assets}"
home_session="${REX_SESSION:-}"
work="$(mktemp -d)"
repo="$work/acme-api"
take="$work/take.mov"

# The cut video, which the GIF is made from.
video="$work/demo.mp4"

# The comment that the demo types into rexamine.
comment="This retries forever. Cap it at 3 attempts with backoff."

# Moves to the retry loop in the diff, which starts on line 6 of the new file.
lines_to_loop=17

# Selects the lines of the retry loop, from line 6 to line 11.
lines_in_loop=5

# Raises the font of both panes, so the text reads well in a scaled-down video.
font_steps=6

key() {
  rex send-key -s "$session" -b "$review" "$1"
  sleep "${2:-0.05}"
}

# Raises the font of the focused pane. The font size belongs to each pane.
raise_font() {
  i=0
  while [ "$i" -lt "$font_steps" ]; do
    rex do client.font.increase >/dev/null
    i=$((i + 1))
  done
}

type_text() {
  printf '%s\n' "$1" | fold -w1 | while IFS= read -r char; do
    rex send -s "$session" -b "$review" -- "${char:- }"
    sleep 0.025
  done
}

# Make a repository where an agent just changed src/api.ts on a branch.
mkdir -p "$repo/src"
cp "$here/fixtures/api.base.ts" "$repo/src/api.ts"
git -C "$repo" init -q -b main
git -C "$repo" add .
git -C "$repo" -c user.name=demo -c user.email=demo@example.com commit -q -m 'feat: add api client'
git -C "$repo" checkout -q -b add-retries
cp "$here/fixtures/api.agent.ts" "$repo/src/api.ts"

created="$(rex new rexamine-demo -c "$repo" --json -- python3 -I "$here/agent.py" "$here/fixtures/api.fixed.ts")"
session="$(printf '%s' "$created" | jq -r .session_id)"
agent="$(printf '%s' "$created" | jq -r '.initial_windows[0].block_ids[0]')"

# Restore shows the sidebar again only when the demo hid it.
sidebar_hidden=0

# Puts the Rex app back as it was, and removes the demo session.
restore_app() {
  # Run every step, so that one failure does not leave the demo on screen.
  set +e
  if [ "$sidebar_hidden" = 1 ]; then
    rex do client.sidebar.toggle >/dev/null
    sidebar_hidden=0
  fi
  [ -n "$home_session" ] && rex do session.select session_id="$home_session" >/dev/null
  rex kill "$session" >/dev/null 2>&1
  set -e
}
trap 'restore_app; rm -rf "$work"' EXIT

# The recording shows the screen, and the font actions need Rex in front.
open -a "Rex Beta"
rex do session.select session_id="$session" >/dev/null
rex do client.sidebar.toggle >/dev/null
sidebar_hidden=1
sleep 0.5
raise_font

# Open rexamine beside the agent, as the key binding does, and raise its font
# before the recording starts. Zooming the agent pane hides rexamine until then.
review="$(rex do -s "$session" -b "$agent" rexamine | jq -r '.block_ids[0]')"
rex focus -s "$session" "$review" >/dev/null
raise_font
rex focus -s "$session" "$agent" >/dev/null

# Give rexamine more width, because the 16:9 crop cuts off its right edge.
rex api call -s "$session" session.resize_pane "$(jq -n --arg agent "$agent" --arg review "$review" \
  '{direction: "horizontal", before_block_id: $agent, after_block_id: $review, ratio: 0.45}')" >/dev/null
rex zoom -s "$session" "$agent" --on >/dev/null

# Wait until the sidebar and font notices are gone.
sleep 3

# ffmpeg records the screen without the mouse pointer, which screencapture always shows.
ffmpeg -v error -y -f avfoundation -capture_cursor 0 -framerate 60 -pixel_format nv12 \
  -i "Capture screen 0:none" -t 17 -c:v libx264 -preset ultrafast -crf 12 "$take" </dev/null 2>/dev/null &
recorder=$!
sleep 1.5

# Show rexamine and focus it, as the key binding does when it opens the pane.
rex zoom -s "$session" "$agent" --off >/dev/null
rex focus -s "$session" "$review" >/dev/null
sleep 1.6

n=0
while [ "$n" -lt "$lines_to_loop" ]; do key j 0.03; n=$((n + 1)); done
sleep 0.4

key v 0.15
n=0
while [ "$n" -lt "$lines_in_loop" ]; do key j 0.12; n=$((n + 1)); done
sleep 0.3

# Write the comment, then send it to the agent and submit it, which focuses the agent.
key c 0.3
type_text "$comment"
sleep 0.6
key ctrl+s

wait "$recorder"
restore_app

# Crop a 16:9 frame from the panes, below their title bars. The default fits a
# Rex window that fills a 16:9 screen, and it cuts off the right edge of rexamine.
size="$(ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=p=0 "$take")"
width="${size%,*}"
height="${size#*,}"
crop_height=$((height * 2000 / 2160 / 2 * 2))
crop_width=$((crop_height * 16 / 9 / 2 * 2))
crop="${DEMO_CROP:-$crop_width:$crop_height:$((width * 14 / 3840)):$((height * 140 / 2160))}"

# Keep the pace of opening and selecting, speed up the typing and the agent's
# answer, and hold on the updated diff. The times match the pauses above.
mkdir -p "$out_dir"
ffmpeg -v error -y -i "$take" -filter_complex "\
[0:v]crop=$crop,scale=1920:-2:flags=lanczos,fps=60,split=4[a][b][c][d];\
[a]trim=1.0:6.0,setpts=PTS-STARTPTS[open];\
[b]trim=6.0:11.3,setpts=(PTS-STARTPTS)/2.2[typing];\
[c]trim=11.3:14.0,setpts=(PTS-STARTPTS)/1.5[answer];\
[d]trim=14.0:16.3,setpts=PTS-STARTPTS[update];\
[open][typing][answer][update]concat=n=4:v=1:a=0,fps=30[v]" \
  -map "[v]" -c:v libx264 -preset slow -crf 18 -pix_fmt yuv420p -movflags +faststart "$video"

ffmpeg -v error -y -i "$video" -vf "\
fps=15,scale=1280:-1:flags=lanczos,split[s0][s1];\
[s0]palettegen=max_colors=128:stats_mode=diff[p];\
[s1][p]paletteuse=dither=none:diff_mode=rectangle" "$out_dir/demo.gif"

echo "rexamine demo: wrote $out_dir/demo.gif"
