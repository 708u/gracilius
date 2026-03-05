#!/usr/bin/env python3
import json
import os
import sys

STATE_DIR = os.path.join(
    os.path.dirname(os.path.abspath(__file__)), "state"
)


def get_state_file(session_id):
    return os.path.join(STATE_DIR, f"{session_id}.json")


def reset_state(session_id):
    state_file = get_state_file(session_id)
    state = {"warned": False}
    try:
        os.makedirs(STATE_DIR, exist_ok=True)
        with open(state_file, "w") as f:
            json.dump(state, f)
    except IOError:
        pass


def main():
    try:
        raw_input = sys.stdin.read()
        input_data = json.loads(raw_input)
    except json.JSONDecodeError:
        sys.exit(0)

    session_id = input_data.get("session_id", "default")
    tool_name = input_data.get("tool_name", "")
    tool_input = input_data.get("tool_input", {})

    if tool_name != "Bash":
        sys.exit(0)

    command = tool_input.get("command", "")
    if not command:
        sys.exit(0)

    if "git push" in command:
        reset_state(session_id)
        print(
            "After pushing, review the entire PR description and "
            "update it to accurately reflect the current state of "
            "all changes in the PR. Ensure consistency across all "
            "sections. Write the PR description in English."
        )

    sys.exit(0)


if __name__ == "__main__":
    main()
