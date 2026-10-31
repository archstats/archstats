# Getting started with Archstats Desktop

## Scan a codebase

1. Open Archstats Desktop. The first screen offers two ways in:
    - **Add Folder** (⌘N / Ctrl+N) picks a folder on your machine. It can be one repository or a folder that holds several.
    - **Clone Repository** (⇧⌘N / Ctrl+Shift+N) clones a git URL first, then adds it.
2. The scan starts on its own. A large repository takes a few minutes; the scan panel in the sidebar shows how far it is.
3. When it finishes, the **Overview** opens on the new snapshot.

Each folder you add is a **workspace**. Every scan of it is kept, so you can go back to an older one or compare two. **Scan Again** (⌘R / Ctrl+R) takes a new snapshot of the code as it is now.

Archstats reads the code where it is. Nothing is uploaded. The AI features are off until you turn them on in Settings, and stay off when your organisation's policy says so.

## Find your way around

The sidebar groups the views by the question they answer:

| Section | Views | Answers |
|---|---|---|
| System | HTTP, CI/CD | What the system serves and how it is built and shipped. Shown when the snapshot holds them. |
| Structure | Units, Libraries, Rules | The modules and units the project declares, what it imports from outside, and which architecture rules it keeps or breaks. |
| Components | Metrics, Hotspots, Connections, Cycles | How the parts depend on each other, where the tangles are, and which parts are both complex and often changed. |
| Code | Files, Find in code | One file at a time, its source and its dependencies. |
| Git | Authors, Activity | Who knows which code, and where the work is going. Muted when the folder has no git history. |
| Tools | Evidence, SQL console | The notes and reports you build, and SQL against the snapshot. |

**Go to Anything** (⌘P / Ctrl+P, or ⇧ twice) jumps to any view, component, file or function by name. **Keyboard Shortcuts** in the Help menu lists the rest.

Every view has a **Show in** menu on a component or file that opens the same thing in another view, and the trail at the top takes you back.

## Slice it your way: groups and lenses

A **group** is a set of components or files you name yourself: a layer, a team's area, a candidate service. Select things in any view and choose **Create group**. A **lens** is a set of groups that cuts the whole codebase one way, such as by layer or by domain. Pick a lens in the sidebar and the views draw your groups instead of the folders.

## Keep and share what you find

- **Export** (⌘E / Ctrl+E) saves the table or figure you are looking at, with where it came from.
- **Evidence** collects pinned findings and turns them into a report you can print to PDF.
- **Save a Copy of Snapshot** writes the snapshot file itself. Anyone with Archstats Desktop or SQLite can open it, and **Import Snapshot** (⌘O / Ctrl+O) opens one you were sent.

## When a snapshot is out of date

A snapshot never changes after it is written. Two things make it stale, and a bar at the top of the view says which:

- **The code moved on.** The checkout has new commits since the scan.
- **The analysis improved.** A newer release reads code differently (a higher *analysis revision*), and the snapshot was written before that.

**Scan Again** in that bar fixes both.

## Where your data lives

| System | Folder |
|---|---|
| macOS | `~/Library/Application Support/archstats` |
| Windows | `%AppData%\archstats` |
| Linux | `~/.config/archstats` |

It holds `app.db` (workspaces, groups, settings), the snapshots under `scans/`, repositories you cloned under `repos/`, report files under `evidence/`, and the app's log under `logs/`. **Settings → About → Show the log** opens the log when something goes wrong; attach it to a [bug report](https://github.com/archstats/archstats-ui/issues).
