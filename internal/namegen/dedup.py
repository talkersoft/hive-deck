#!/usr/bin/env python3
"""Sort and deduplicate adjectives.txt and nouns.txt in place."""

import os

files = ["adjectives.txt", "nouns.txt"]
dir = os.path.dirname(os.path.abspath(__file__))

for filename in files:
    path = os.path.join(dir, filename)
    with open(path) as f:
        words = {line.strip().lower() for line in f if line.strip()}
    sorted_words = sorted(words)
    with open(path, "w") as f:
        f.write("\n".join(sorted_words) + "\n")
    print(f"{filename}: {len(sorted_words)} words")
