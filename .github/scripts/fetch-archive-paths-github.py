#!/usr/bin/python3
"""List blob paths under a GitHub repo tree via REST API (no git clone).

Works anonymously on public repos. Set GITHUB_TOKEN for higher rate limits.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.request


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__.split("\n", 1)[0])
    p.add_argument(
        "output",
        help="File to write (one repo-relative path per line)",
    )
    p.add_argument(
        "prefixes",
        nargs="+",
        metavar="PREFIX",
        help="Path prefixes to include (e.g. amzn/ centos/)",
    )
    p.add_argument(
        "--owner",
        default=os.environ.get("GITHUB_ARCHIVE_OWNER", "aquasecurity"),
    )
    p.add_argument(
        "--repo",
        default=os.environ.get("GITHUB_ARCHIVE_REPO", "btfhub-archive"),
    )
    p.add_argument(
        "--branch",
        default=os.environ.get("GITHUB_ARCHIVE_BRANCH", "main"),
    )
    args = p.parse_args()

    def get_json(url: str) -> dict:
        req = urllib.request.Request(
            url,
            headers={
                "Accept": "application/vnd.github+json",
                "X-GitHub-Api-Version": "2022-11-28",
            },
        )
        token = os.environ.get("GITHUB_TOKEN")
        if token:
            req.add_header("Authorization", f"Bearer {token}")
        try:
            with urllib.request.urlopen(req, timeout=120) as resp:
                return json.load(resp)
        except urllib.error.HTTPError as e:
            body = e.read().decode("utf-8", errors="replace")
            print(f"HTTP {e.code} from {url}: {body}", file=sys.stderr)
            raise SystemExit(1) from e

    base = f"https://api.github.com/repos/{args.owner}/{args.repo}"
    commit = get_json(f"{base}/commits/{args.branch}")
    tree_sha = commit["commit"]["tree"]["sha"]
    tree = get_json(f"{base}/git/trees/{tree_sha}?recursive=1")

    if tree.get("truncated"):
        print(
            "error: git tree response truncated; narrow prefixes or paginate",
            file=sys.stderr,
        )
        return 2

    paths: list[str] = []
    for item in tree.get("tree") or []:
        if item.get("type") != "blob":
            continue
        path = item.get("path") or ""
        if any(path.startswith(pref) for pref in args.prefixes):
            paths.append(path)

    out_dir = os.path.dirname(os.path.abspath(args.output))
    if out_dir:
        os.makedirs(out_dir, exist_ok=True)

    lines = [p + "\n" for p in sorted(set(paths))]
    out_tmp = args.output + ".tmp"
    try:
        with open(out_tmp, "w", encoding="utf-8") as f:
            f.writelines(lines)
        os.replace(out_tmp, args.output)
    except OSError:
        if os.path.exists(out_tmp):
            os.unlink(out_tmp)
        raise

    print(len(lines), args.output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
