# Install

## Archstats Desktop

Download the file for your system from the [latest release](https://github.com/archstats/archstats-ui/releases/latest). The names below are from v0.12.0; yours carry the version you download.

| System | File | Notes |
|---|---|---|
| macOS 10.13 or later, Apple silicon or Intel | `archstats-desktop-v0.12.0-macos-universal.dmg` | Open it and drag the app to Applications. A `.zip` of the same app is there too. |
| Windows 10 or later, 64-bit | `archstats-desktop-v0.12.0-windows-amd64-installer.exe` | Or the `-portable.zip`, which runs without installing. |
| Debian, Ubuntu | `archstats-desktop_0.12.0_amd64.deb` (or `_arm64`) | `sudo apt install ./archstats-desktop_*.deb` installs it with the GTK and WebKit libraries it needs. |
| Fedora, RHEL | `archstats-desktop-0.12.0-1.x86_64.rpm` (or `.aarch64`) | `sudo dnf install ./archstats-desktop-*.rpm` |
| Other Linux | `archstats-desktop-v0.12.0-linux-amd64.tar.gz` (or `arm64`) | Needs GTK 3 and WebKitGTK 4.1 installed. |

### The first launch of an unsigned build

The builds are not signed with an Apple or Microsoft certificate yet, so both systems ask before the first launch.

**macOS.** Opening the app the first time shows *"Archstats Desktop" can't be opened* or *Apple could not verify…*. To open it anyway:

1. Click **Done** (or **OK**) in that dialog.
2. Open **System Settings → Privacy & Security**.
3. Scroll to **Security**. Next to the message about Archstats Desktop, click **Open Anyway**, and confirm with your password.

You do this once per version. On macOS 14 and earlier, right-click the app in Finder and choose **Open** instead.

**Windows.** SmartScreen shows *Windows protected your PC*. Click **More info**, then **Run anyway**.

### Check the download

Every release has a `checksums.txt` with the SHA-256 of each file:

```shell
shasum -a 256 -c checksums.txt --ignore-missing     # macOS
sha256sum -c checksums.txt --ignore-missing         # Linux
```

On Windows, compare the output of `Get-FileHash .\archstats-desktop-*-installer.exe` with the line in `checksums.txt`.

### Updates

The app tells you when a newer release is out, and shows what changed the first time you open a new version. See [Updates and releases](desktop/updates.md).

## The `archstats` CLI

With Go 1.23 or later:

```shell
go install github.com/archstats/archstats@latest
```

Or download an archive for your system from the [latest release](https://github.com/archstats/archstats/releases/latest): Linux (amd64, arm64), macOS (arm64, amd64) and Windows (amd64). Unpack it and put `archstats` on your `PATH`.

A binary downloaded with a browser on macOS is quarantined. Clear that once:

```shell
xattr -d com.apple.quarantine ./archstats
```

Check it runs:

```shell
archstats --version
```

Then go on to [Commands](cli.md).
