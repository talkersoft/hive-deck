#!/usr/bin/env python3
"""Merge dumped word lists into the master adjectives.txt / nouns.txt files.

Drop any number of .txt files into the adjectives/ or nouns/ folders. Running
this script merges every word from those dumps into the matching master file,
skipping words that already exist. A dump file is deleted only after every one
of its words is confirmed present in the master file.
"""

import os

DIR = os.path.dirname(os.path.abspath(__file__))

# (dump folder, master file)
CATEGORIES = [
    ("adjectives", "adjectives.txt"),
    ("nouns", "nouns.txt"),
]


def load_words(path):
    if not os.path.exists(path):
        return []
    with open(path) as f:
        return [line.strip().lower() for line in f if line.strip()]


def write_master(path, words):
    with open(path, "w") as f:
        f.write("\n".join(sorted(words)) + "\n")


def merge_category(folder_name, master_name):
    folder = os.path.join(DIR, folder_name)
    master_path = os.path.join(DIR, master_name)

    if not os.path.isdir(folder):
        return

    master_words = set(load_words(master_path))

    dump_files = sorted(
        f for f in os.listdir(folder) if f.lower().endswith(".txt")
    )
    if not dump_files:
        print(f"{master_name}: no dump files in {folder_name}/")
        return

    for dump in dump_files:
        dump_path = os.path.join(folder, dump)
        dump_words = load_words(dump_path)

        added = sorted(w for w in set(dump_words) if w not in master_words)
        master_words.update(dump_words)

        # Rewrite the master before deleting so we never lose words.
        write_master(master_path, master_words)

        # Confirm every word from the dump is now in the master record.
        confirmed = set(load_words(master_path))
        missing = [w for w in dump_words if w not in confirmed]

        if missing:
            print(
                f"  ! {folder_name}/{dump}: {len(missing)} words missing after "
                f"merge, keeping file. Examples: {missing[:5]}"
            )
            continue

        os.remove(dump_path)
        print(
            f"  + {folder_name}/{dump}: added {len(added)} new word(s), "
            f"removed dump file"
        )

    print(f"{master_name}: {len(master_words)} words total")


def main():
    for folder_name, master_name in CATEGORIES:
        merge_category(folder_name, master_name)


if __name__ == "__main__":
    main()
