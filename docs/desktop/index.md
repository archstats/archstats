# Get started

From download to your first picture of a codebase takes a few minutes. This page goes in order: install the app, scan a folder, read what comes back, and keep it up to date.

## Install {#install}

Download the file for your system from the [latest release](https://github.com/archstats/archstats-ui/releases/latest). The names below are from v0.12.0; yours carry the version you download.

| System | File | Then |
|---|---|---|
| macOS 10.13 or later, Apple silicon or Intel | `…-macos-universal.dmg` | Open it and drag the app to Applications. |
| Windows 10 or later, 64-bit | `…-windows-amd64-installer.exe` | Run it. Or take the `-portable.zip`, which runs without installing. |
| Debian, Ubuntu | `archstats-desktop_…_amd64.deb` (or `_arm64`) | `sudo apt install ./archstats-desktop_*.deb` |
| Fedora, RHEL | `archstats-desktop-…x86_64.rpm` (or `.aarch64`) | `sudo dnf install ./archstats-desktop-*.rpm` |
| Other Linux | `…-linux-amd64.tar.gz` (or `arm64`) | Unpack it. It needs GTK 3 and WebKitGTK 4.1. |

### The first launch

The builds are not signed with an Apple or Microsoft certificate yet, so both systems ask once before the app opens.

- **macOS** says *"Archstats Desktop" can't be opened*. Click **Done**, open **System Settings → Privacy & Security**, scroll to **Security** and click **Open Anyway** next to Archstats Desktop. On macOS 14 and earlier, right-click the app in Finder and choose **Open** instead. You do this once per version.
- **Windows** says *Windows protected your PC*. Click **More info**, then **Run anyway**.

To check a download, compare it with `checksums.txt` in the same release:

```shell
shasum -a 256 -c checksums.txt --ignore-missing     # macOS
sha256sum -c checksums.txt --ignore-missing         # Linux
```

On Windows, compare `Get-FileHash .\archstats-desktop-*-installer.exe` with its line in `checksums.txt`.

## Scan a codebase {#scan}

The first screen offers two ways in:

- **Add Folder** (⌘N / Ctrl+N) picks a folder on your machine: one repository, or a folder that holds several.
- **Clone Repository** (⇧⌘N / Ctrl+Shift+N) clones a git address with your own git credentials, then adds it.

The scan starts on its own. A large repository takes a few minutes, and the sidebar shows how far it is. Archstats reads the code where it is: nothing is uploaded, and there is no account.

Each folder you add is a **workspace**. Every scan of it is kept as a **snapshot**, so you can go back to an older one or compare two. **Scan Again** (⌘R / Ctrl+R) takes a new snapshot of the code as it is now.

## Read what comes back {#read}

When the scan finishes, the **Overview** opens.

<Shot name="overview" title="Overview" source="Apache Fineract">
The size of the codebase, its health, its tangles and its history, each linking into the view that explains it.
</Shot>

The sidebar groups the other views by the question they answer:

| Section | Views | Answers |
|---|---|---|
| System | HTTP, CI/CD | What the system serves, and how it is built and shipped. |
| Structure | Connections, Cycles, Rules | How the parts depend on each other, and where they tangle. |
| Components | Metrics, Hotspots | Every number, and the code that costs the most to keep. |
| Code | Units, Libraries | The layers the framework declares, and what comes from outside. |
| Git | Activity, Authors | Where the work goes, and who still knows each part. |
| Tools | Evidence, SQL console | Your reports, and SQL against the snapshot. |

The [Tour of the app](../tour.md) shows each view with a picture and how to read it.

A few things work the same everywhere:

- **Click** a component or file to inspect it; **double-click** to open its page.
- **⌘P** (Ctrl+P), or **⇧** twice, jumps to any view, component, file or function by name.
- **Export** (⌘E / Ctrl+E) saves the table or figure in front of you, with the snapshot it came from.

## Stay up to date {#updates}

The app checks GitHub for a newer release when it starts and once a day after. When there is one, a note in the bottom-left corner offers **Download**. The check is an anonymous read of a public page; turn it off in **Settings → Check for updates**. To update, download the new version and replace the app. Your workspaces, snapshots and settings stay where they are. The first time you open a new version, **What's new** lists what changed.

A snapshot never changes after it is written. A bar at the top of a view says when one is out of date, and why:

- **The code moved on**: the checkout has commits the scan did not see.
- **The analysis improved**: a newer release reads code differently (a higher *analysis revision*) than the one that wrote the snapshot.

**Scan Again** in that bar fixes both.

## Where your data lives {#data}

| System | Folder |
|---|---|
| macOS | `~/Library/Application Support/archstats` |
| Windows | `%AppData%\archstats` |
| Linux | `~/.config/archstats` |

It holds `app.db` (workspaces, groups, settings), the snapshots under `scans/`, repositories you cloned under `repos/`, report files under `evidence/`, and the app's log under `logs/`. Each snapshot is a plain SQLite file: **Save a Copy of Snapshot** writes one anywhere, and **Import Snapshot** (⌘O / Ctrl+O) opens one you were sent. [Snapshot tables](../snapshot.md) describes what is in it.

When something goes wrong, **Settings → About → Show the log** opens the log; attach it to a [bug report](https://github.com/archstats/archstats-ui/issues).
