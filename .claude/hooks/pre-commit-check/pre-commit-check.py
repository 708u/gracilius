#!/usr/bin/env python3
import json
import os
import random
import sys
from datetime import datetime

CHECKS = {
    "code": {
        "when": "When Go code (*.go, go.mod, go.sum) is modified",
        "commands": [
            "make test",
            "make lint",
            "make fmt",
            "go mod tidy",
        ],
    },
    "build": {
        "when": "When Go code is modified",
        "commands": [
            "make build",
        ],
    },
    "test_quality": {
        "when": "When new code or logic changes are added",
        "checks": [
            "Tests exist for all new/changed exported functions",
            "Happy path: typical inputs produce expected outputs",
            "Edge cases: nil, empty, zero values handled",
            "Boundary values: range limits (0, 1, max-1, max)",
            "Error paths: invalid input, missing resources, "
            "permission errors",
        ],
    },
}

STATE_DIR = os.path.join(
    os.path.dirname(os.path.abspath(__file__)), "state"
)


def get_state_file(session_id):
    return os.path.join(STATE_DIR, f"{session_id}.json")


def cleanup_old_state_files():
    try:
        if not os.path.exists(STATE_DIR):
            return
        current_time = datetime.now().timestamp()
        thirty_days_ago = current_time - (30 * 24 * 60 * 60)
        for filename in os.listdir(STATE_DIR):
            if filename.endswith(".json"):
                file_path = os.path.join(STATE_DIR, filename)
                try:
                    file_mtime = os.path.getmtime(file_path)
                    if file_mtime < thirty_days_ago:
                        os.remove(file_path)
                except (OSError, IOError):
                    pass
    except Exception:
        pass


def load_state(session_id):
    state_file = get_state_file(session_id)
    if os.path.exists(state_file):
        try:
            with open(state_file, "r") as f:
                data = json.load(f)
                return {
                    "warned": data.get("warned", False),
                    "pushed": data.get("pushed", False),
                }
        except (json.JSONDecodeError, IOError):
            pass
    return {"warned": False, "pushed": False}


def save_state(session_id, state):
    state_file = get_state_file(session_id)
    try:
        os.makedirs(STATE_DIR, exist_ok=True)
        with open(state_file, "w") as f:
            json.dump(state, f)
    except IOError:
        pass


def main():
    if random.random() < 0.1:
        cleanup_old_state_files()

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

    if "git commit" not in command:
        sys.exit(0)

    state = load_state(session_id)

    if state["warned"]:
        sys.exit(0)

    state["warned"] = True
    save_state(session_id, state)

    print(
        "BLOCKED: Required checks not confirmed.",
        file=sys.stderr,
    )
    print("", file=sys.stderr)
    print("Run before committing:", file=sys.stderr)
    for category, info in CHECKS.items():
        print(
            f"  [{category}] {info['when']}",
            file=sys.stderr,
        )
        items = info.get("commands", []) + info.get("checks", [])
        for item in items:
            print(f"    - {item}", file=sys.stderr)
    print("", file=sys.stderr)
    print(
        "After committing and pushing, review the entire PR "
        "description and update it to accurately reflect the "
        "current state of all changes in the PR. Ensure "
        "consistency across all sections. Write the PR "
        "description in English.",
        file=sys.stderr,
    )
    print("", file=sys.stderr)
    print(
        "After running checks, commit again to confirm.",
        file=sys.stderr,
    )
    sys.exit(2)


if __name__ == "__main__":
    main()
