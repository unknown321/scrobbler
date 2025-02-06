Scrobbler
=========

Scrobbler for Linux-based NW Series WALKMAN® portable players.

Creates `.scrobbler.log` on internal storage.

### Device support

| Device          | Stock | Walkman One | Notes                          |
|-----------------|-------|-------------|--------------------------------|
| NW-A50          | ✅     | ✅           |                                |
| └──NW-A50Z      | ✅     | n/a         | mod is unavailable             |
| NW-A40          | ✅     | ✅           | community tested               |
| └──[A50 mod][1] | ✅     | n/a         |                                |
| NW-A30          | ?     | ?           | build available, needs testing |
| NW-ZX300        | ?     | ?           | build available, needs testing |
| NW-WM1A/Z       | ?     | ?           | build available, needs testing |
| DMP-Z1          | ?     | n/a         | build available, needs testing |

[1]: https://www.mrwalkman.com/p/nw-a40-stock-update.html

### Build

#### Requirements
- make
- Go >= 1.22.1

```shell
make build
```

### Release

```shell
git submodule update --recursive --init
make release
```

Grab Windows installer/UPG files from `release` directory.

### Installation

See [INSTALL.md](./INSTALL.md)

### Usage
Before you start, `Device Settings -> Beep Settings` option __must__ be turned off:

<img src="images/beep.png" height="400" alt="beep switch location">

Why? Beeps are inserted in play queue as regular tracks; it resets currently played track.

After that just play some tracks and check for `.scrobbler.log` in root directory on your device.

### See also

https://github.com/unknown321/wampy
