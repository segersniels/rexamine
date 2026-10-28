"""A stand-in coding agent for the rexamine demo.

It shows the end of a finished turn. When it gets a pasted review comment and
Enter, it answers, and then copies the fixed file over src/api.ts in the
current directory, so the diff in rexamine updates.

Usage: python3 -I agent.py <fixed file>
"""

import os
import shutil
import sys
import termios
import time
import tty

DIM, BOLD, RESET = "\x1b[2m", "\x1b[1m", "\x1b[0m"
GREEN, RED, MAGENTA, CYAN = "\x1b[32m", "\x1b[31m", "\x1b[35m", "\x1b[36m"

# Bracketed paste puts these around the text that rexamine pastes.
PASTE_START, PASTE_END = b"\x1b[200~", b"\x1b[201~"


def out(text=""):
    """Writes one line in raw mode, where a newline does not return the cursor."""
    sys.stdout.write(text.replace("\n", "\r\n") + "\r\n")
    sys.stdout.flush()


def prompt():
    """Writes the input prompt."""
    sys.stdout.write(f"{MAGENTA}{BOLD}❯{RESET} ")
    sys.stdout.flush()


def show_turn():
    """Shows the finished turn that the demo starts from."""
    sys.stdout.write("\x1b[2J\x1b[H")
    out(f"{DIM}agent · {os.path.basename(os.getcwd())}{RESET}")
    out()
    out(f"{MAGENTA}{BOLD}❯{RESET} {BOLD}Add retries to fetchUser{RESET}")
    out()
    out(f"{CYAN}●{RESET} Edited {BOLD}src/api.ts{RESET}  {GREEN}+9{RESET} {RED}-3{RESET}")
    out("  fetchUser now retries when the request fails.")
    out()
    prompt()


def answer(fixed_file):
    """Answers the review comment, and then applies the fix."""
    out()
    out()
    for frame in "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏" * 2:
        sys.stdout.write(f"\r{CYAN}{frame}{RESET} {DIM}Thinking…{RESET}")
        sys.stdout.flush()
        time.sleep(0.07)
    sys.stdout.write("\r\x1b[2K")
    out(f"{CYAN}●{RESET} Good catch, it retries forever. I'll stop after 3 attempts")
    out("  and wait longer between each one.")

    time.sleep(0.6)
    shutil.copyfile(fixed_file, "src/api.ts")
    out()
    out(f"{CYAN}●{RESET} Edited {BOLD}src/api.ts{RESET}")
    out()
    prompt()


def main():
    fixed_file = os.path.abspath(sys.argv[1])
    fd = sys.stdin.fileno()
    saved = termios.tcgetattr(fd)
    tty.setraw(fd)
    sys.stdout.write("\x1b[?2004h")
    show_turn()

    buffer, pasting, pasted = b"", False, ""
    try:
        while True:
            buffer += os.read(fd, 1024)
            while buffer:
                if pasting:
                    end = buffer.find(PASTE_END)
                    if end < 0:
                        break
                    pasted += buffer[:end].decode()
                    buffer, pasting = buffer[end + len(PASTE_END) :], False

                    # Show the pasted comment in the prompt, as an agent does.
                    sys.stdout.write(pasted.rstrip("\n").replace("\r\n", "\n").replace("\n", "\r\n  "))
                    sys.stdout.flush()
                elif buffer.startswith(PASTE_START):
                    buffer, pasting = buffer[len(PASTE_START) :], True
                elif buffer[:1] in (b"\r", b"\n"):
                    buffer = buffer[1:]
                    if pasted:
                        answer(fixed_file)
                        pasted = ""
                elif buffer[:1] == b"\x03":
                    return
                else:
                    buffer = buffer[1:]
    finally:
        sys.stdout.write("\x1b[?2004l")
        termios.tcsetattr(fd, termios.TCSADRAIN, saved)


main()
