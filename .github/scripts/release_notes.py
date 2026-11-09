"""Validate root-module release metadata without publishing anything."""

import datetime
import os
from pathlib import Path
import re
import subprocess


VERSION = re.compile(
    r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
    r"(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?"
    r"(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?"
)


def parse_tag(tag):
    match = VERSION.fullmatch(tag)
    if match is None:
        raise ValueError("tag must be a root-module SemVer tag")
    prerelease = match[4]
    if prerelease:
        for part in prerelease.split("."):
            if part.isascii() and part.isdigit() and len(part) > 1 and part[0] == "0":
                raise ValueError("numeric prerelease identifiers must not have leading zeros")
    return int(match[1]), bool(prerelease)


def validate_module(tag, go_mod):
    major, is_prerelease = parse_tag(tag)
    match = re.search(r"^module\s+(\S+)\s*$", go_mod, re.MULTILINE)
    if match is None:
        raise ValueError("module declaration is missing")
    suffix = re.search(r"/v([0-9]+)$", match[1])
    if major >= 2:
        if suffix is None or int(suffix[1]) != major:
            raise ValueError("tag major does not match module path")
    elif suffix is not None:
        raise ValueError("v0/v1 tags require an unsuffixed module path")
    # Preserve this repository's policy: all v0 releases are prereleases.
    return major == 0 or is_prerelease


def extract_notes(tag, changelog):
    version = tag[1:]
    prefix = f"## [{version}]"
    lines = changelog.splitlines()
    matches = [i for i, line in enumerate(lines)
               if line == prefix or line.startswith(prefix + " ")]
    if len(matches) != 1:
        raise ValueError("changelog must have exactly one matching version section")
    start = matches[0]
    heading = lines[start]
    # Existing job releases allow undated headings. When present, dates must
    # be real ISO dates; Unreleased is never a publication-ready heading.
    suffix = heading[len(prefix):]
    if suffix:
        if not re.fullmatch(r" - [0-9]{4}-[0-9]{2}-[0-9]{2}", suffix):
            raise ValueError("changelog heading is not ready for release")
        datetime.date.fromisoformat(suffix[3:])
    end = next((i for i in range(start + 1, len(lines))
                if lines[i].startswith("## ")), len(lines))
    body = lines[start + 1:end]
    # Link definitions are document metadata, not release notes.
    body = [line for line in body if not re.match(r"^\[[^]]+\]:", line)]
    if not any(line.strip() and not line.lstrip().startswith("#") for line in body):
        raise ValueError("changelog section has no release notes")
    return "\n".join(body).strip() + "\n"


def previous_tag(tag, go_mod):
    parent = subprocess.run(
        ["git", "rev-parse", "--verify", f"{tag}^"],
        capture_output=True, text=True,
    )
    if parent.returncode:
        return None
    tags = subprocess.check_output(
        ["git", "tag", "--merged", parent.stdout.strip(), "--sort=-version:refname"],
        text=True,
    ).splitlines()
    for candidate in tags:
        try:
            validate_module(candidate, go_mod)
        except ValueError:
            continue
        return candidate
    return None


def main():
    tag = os.environ["GITHUB_REF_NAME"]
    go_mod = Path("go.mod").read_text()
    is_prerelease = validate_module(tag, go_mod)
    notes = extract_notes(tag, Path("CHANGELOG.md").read_text())
    previous = previous_tag(tag, go_mod)
    if previous:
        base = os.environ["GITHUB_SERVER_URL"] + "/" + os.environ["GITHUB_REPOSITORY"]
        notes += f"\n**Full Changelog**: {base}/compare/{previous}...{tag}\n"
    (Path(os.environ["RUNNER_TEMP"]) / "job-release-notes.md").write_text(notes)
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write(f"is_prerelease={str(is_prerelease).lower()}\n")


if __name__ == "__main__":
    main()
