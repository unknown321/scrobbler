Nice portable devices record play counts and timestamps, like iPod or any player with Rockbox.

With that info you can scrobble your tracks somewhere by following [audioscrobbler spec](https://web.archive.org/web/20170107015006/http://www.audioscrobbler.net/wiki/Portable_Player_Logging).

Walkman NW-A50 is not nice.

## Scrobbling requirements

  - getting current track info: artist, title, duration
  - getting player status: playing/stopped

## Possible approaches
<!-- TOC -->
  * [Asking running services about current track](#asking-running-services-about-current-track)
    * [Getting between client and service via binder](#getting-between-client-and-service-via-binder)
    * [Code injection](#code-injection)
      * [LD_PRELOAD tricks](#ldpreload-tricks)
      * [Patching HgrmMediaPlayerApp / libPlayerService to run other code](#patching-hgrmmediaplayerapp--libplayerservice-to-run-other-code)
      * [libMediaStoreService.so LD_PRELOAD injection](#libmediastoreserviceso-ldpreload-injection)
  * [Using database](#using-database)
    * [SQLite triggers](#sqlite-triggers)
  * [Watching for open files](#watching-for-open-files)
    * [Getting track data directly from files](#getting-track-data-directly-from-files)
  * [Reading logs](#reading-logs)
    * [Logcat](#logcat)
      * [Parsing logs](#parsing-logs)
        * [Reading from logcat](#reading-from-logcat)
        * [Reading directly from kernel buffer](#reading-directly-from-kernel-buffer)
<!-- TOC -->

## Asking running services about current track

Not possible. Services communicate over [binder interface](https://developer.android.com/reference/android/os/Binder);
one session per client.
It is possible to connect to running service, but you'll get a new empty session instead of already running.

### Getting between client and service via binder

You can recompile binder kernel module to print info or just read binder log to get a peek at what's happening; you'll get 
function id and a pointer to _binder parcel_, which has address of whatever data is passed.

Example (https://stackoverflow.com/a/39328585/1938027):

```shell
$ cd /sys/kernel/debug/tracing
$ echo > set_event  # clear all unrelated events
$ echo 1 > events/binder/enable
$ echo 1 > tracing_on

# .. do your test jobs ..

$ cat trace

# refer to https://android.googlesource.com/kernel/common/+/android-3.10.y/Documentation/trace/ftrace.txt for more detail info.
```

There is no real data, just pointers.

### Code injection

#### LD_PRELOAD tricks

You can inject your C++ code into HgrmMediaPlayerApp, libPlayerService or libPlayerServiceClient.
By hooking some promising functions like `pst::services::playerservice::PlayController::GetCurrentStatus` 
you can get pointer to `status` object.

Status object is complex and consists of nested structures; decompiling those takes too much time.

Possible, but not viable.

#### Patching HgrmMediaPlayerApp / libPlayerService to run other code

Carefully patch binary to `jmp` somewhere else when needed and `ret` back.
Most likely `CurrentStatus` functions or `OnNextTrack`?

Firmware-dependent, possibility of breaking current code, have to reverse engineer a lot of structures.

Not viable.

#### libMediaStoreService.so LD_PRELOAD injection

Inject C code before `SELECT` function, or before submitting the query. Read the query, run it again, get track info.

Haven't tried this, but there are problems:
- query results might be cached; no repeated select if track is looped (workaround is possible if player state is available)
- `select` runs during library browsing, might be getting many selects even if track is unchanged
- `select` runs before the end of current track to prepare next track; there is no guarantee this track would be played

Promising, but complicated and unreliable.


## Using database

Database software is called genesys-db, some kind of SQLite fork. You can `adb pull` it from device, /db/MTPDB.dat.

Database provides track data and, possibly, track change data.

### SQLite triggers

The idea is to make database write something into itself on `select * from object_body ...` query.

However, triggers can be created only for DELETE, INSERT, UPDATE - https://www.sqlite.org/lang_createtrigger.html

Not possible.

## Watching for open files

Process `/system/vendor/sony/bin/hagodaemon MediaStoreService PlayerService` opens currently played music file:

```shell
$ pid=$(ps ax | grep "[h]agodaemon MediaStore" | cut -d' ' -f3 | sort | tail -1)
$ ls -l /proc/$pid/fd/* | grep contents
lr-x------ system   system            2024-04-15 03:06 9 -> /contents_ext/music/my/music/file.mp3
```

However:
- might be next track preparation; no guarantees that it would be played
- file is always opened no matter the player status


### Getting track data directly from files

Possible, but unreliable. You'll have to use [TagLib](https://taglib.org/),
[id3v2lib](https://github.com/larsbs/id3v2lib) or something else mature enough to be able to work with all formats and tags.

Duration is rarely present in tags, so you'll have to calculate it yourself.

Currently, there is no mature Go library for ID3v2.


## Reading logs

### Logcat

https://developer.android.com/tools/logcat

Services and player write logs with current player status and file path.

You can read the logs by executing logcat utility or by reading log buffer yourself.

Logs are unreliable as a source of truth, but it's the most straightforward option out of all mentioned above.

#### Parsing logs

Simplified log:
```shell
content URI: /data/file.mp3
status: [stopped]
status: [paused]
status: [playing]
```

Issues:
  - incorrect order: there is no guarantee if `write play status` thread works after `write content uri`. `Playing` status might be related to previous track.
  - information duplication: sometimes content uri might be duplicated (how?)
  - too many logs (see `reading from logcat` below)

##### Reading from logcat

Example https://github.com/google/gapid/blob/11e56ff/core/os/android/adb/logcat.go

Poor performance due to constant process spawning.

Services may write too many logs, so your buffer has to be huge (hundreds of lines).
Even then there is a risk of valuable lines (with status and content) not getting into buffer.

Having to parse same records again unless you flush everything after reading with `logcat -c`, which I consider intrusive.

Viable, but clunky.

##### Reading directly from kernel buffer

See source code.