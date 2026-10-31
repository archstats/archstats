# Updates and releases

## When a new release is out

Archstats Desktop asks GitHub for the latest published release when it starts, and once a day while it stays open. When there is a newer one, a note in the bottom-left corner says so, with **Download**, which opens the release page, and **Not now**, which hides the note until the release after.

The request is an anonymous read of a public page: nothing about you, your machine or your code is sent. Turn it off in **Settings → Check for updates**. **Help → Check for Updates…** (or **Check now** in Settings) asks right away.

Builds that are not releases, such as one you built from source, never offer an update.

Installing a new version is the same as the first install: download it and replace the app. Your workspaces, snapshots and settings stay where they are.

## What's new

The first time you open a new version, **What's new** lists the notes of every release since the one you ran before. **Help → What's New** shows this version's notes again. The notes come with the app, so they work offline, and are in English.

## Analysis revisions and old snapshots

Each release of the engine inside the app has an **analysis revision**: a number that goes up whenever the engine reads code differently, such as a dependency it now resolves or a count it corrected. Release notes say when it changed.

A snapshot keeps the revision it was written with. When you open one older than the app's, a bar says what has changed since and offers **Scan Again**. Until you rescan, the snapshot shows what the older analysis read.

**Settings → About** shows the app's version, the engine it carries and its analysis revision.

## Release notes

Every release's notes are in the changelog:

- [Archstats Desktop changelog](https://github.com/archstats/archstats-ui/blob/main/CHANGELOG.md)
- [Engine and CLI changelog](../releases.md)
