// Shared GROL terminal support: localStorage-backed VFS, stdin/stdout/stderr
// bridge, and WASM initialization. Used by both xterm.html and ghostty.html.
//
// Call setupGrolTerminal(xterm, config) after creating the terminal instance
// and setting globalThis.TerminalConnected/Cols/Rows.
//
// config fields:
//   versionSuffix  - string appended to version display (default '')
//   onImageShow    - callback when image is displayed (e.g., fitAddon.fit())
//   onImageClose   - callback when image close button clicked
//   safeWrite      - wrap xterm.write() in try/catch (for ghostty-web)
//   copyBuffer     - copy buffer before decoding in writeSync (for ghostty-web)

// eslint-disable-next-line no-unused-vars
function setupGrolTerminal(xterm, config) {
    config = config || {};
    const versionSuffix = config.versionSuffix || '';
    const onImageShow = config.onImageShow || function() {};
    const onImageClose = config.onImageClose || function() {};
    const safeWrite = config.safeWrite || false;
    const copyBuffer = config.copyBuffer || false;

    const textEncoder = new TextEncoder();
    const textDecoder = new TextDecoder();

    // Terminal write wrapper — optionally wraps in try/catch for ghostty-web
    // whose WASM-based write() can throw (e.g., "offset is out of bounds").
    function termWrite(data) {
        if (safeWrite) {
            try {
                xterm.write(data);
            } catch (e) {
                console.error('terminal write failed:', e);
                console.log(data);
            }
        } else {
            xterm.write(data);
        }
    }

    // Export closeImage to global scope for the onclick handler
    globalThis.closeImage = function() {
        document.getElementById('image-container').style.display = 'none';
        onImageClose();
    };

    // ─── localStorage-backed Virtual Filesystem ─────────────────
    //
    // Go WASM uses globalThis.fs for all file I/O (via wasm_exec.js).
    // We provide a minimal in-memory filesystem backed by localStorage
    // so that REPL history (~/.grol_history) and state (.gr) persist
    // across browser sessions. Files are stored as localStorage keys
    // prefixed with "grol:vfs:".
    //
    // Also bridges stdin/stdout/stderr:
    //   - os.Stdout (fd=1) → xterm.write()
    //   - os.Stderr (fd=2) → xterm.write()
    //   - os.Stdin  (fd=0) ← xterm.onData()

    // Set proper POSIX fs.constants (wasm_exec.js defaults them to -1).
    // Go's syscall/fs_js.go reads these at init time to map Go flags
    // (O_WRONLY, O_CREATE, etc.) to the values passed to fs.open().
    globalThis.fs.constants = {
        O_RDONLY:    0,
        O_WRONLY:    1,
        O_RDWR:     2,
        O_CREAT:    0o100,
        O_EXCL:     0o200,
        O_TRUNC:    0o1000,
        O_APPEND:   0o2000,
        O_DIRECTORY: 0o200000,
    };

    // ─── Virtual File System (localStorage-backed) ───────────────
    const VFS_PREFIX = "grol:vfs:";
    const S_IFREG = 0o100000;

    // In-memory file store: path → content string
    // Loaded from localStorage on open, saved back on close.
    const vfsFiles = {};    // path → string content
    const vfsOpenFDs = {};  // fd → { path, pos, flags, data (Uint8Array) }
    let vfsNextFD = 10;     // start above stdin/stdout/stderr

    function vfsLoad(path) {
        const key = VFS_PREFIX + path;
        const data = localStorage.getItem(key);
        if (data !== null) {
            vfsFiles[path] = data;
            return true;
        }
        return path in vfsFiles;
    }

    function vfsSave(path) {
        const key = VFS_PREFIX + path;
        if (path in vfsFiles) {
            localStorage.setItem(key, vfsFiles[path]);
        }
    }

    function vfsDelete(path) {
        delete vfsFiles[path];
        localStorage.removeItem(VFS_PREFIX + path);
    }

    // Normalize paths: strip leading "./", resolve "path.resolve" quirks
    function vfsNormalize(p) {
        if (typeof p !== 'string') return p;
        // Remove leading ./
        while (p.startsWith('./')) p = p.substring(2);
        // Remove leading /
        while (p.startsWith('/')) p = p.substring(1);
        return p;
    }

    function vfsMakeStat(path, isDir) {
        const content = vfsFiles[path] || '';
        const size = isDir ? 0 : textEncoder.encode(content).length;
        const ms = Date.now();
        return {
            dev: 0, ino: 0,
            mode: isDir ? (0o40000 | 0o755) : (S_IFREG | 0o644),
            nlink: 1, uid: 0, gid: 0, rdev: 0,
            size: size,
            blksize: 4096, blocks: Math.ceil(size / 512),
            atimeMs: ms, mtimeMs: ms, ctimeMs: ms,
            isDirectory() { return isDir; },
        };
    }

    // Override fs.open
    const origFsOpen = globalThis.fs.open.bind(globalThis.fs);
    globalThis.fs.open = function(path, flags, mode, callback) {
        path = vfsNormalize(path);
        const O = globalThis.fs.constants;
        const accessMode = flags & 3; // O_RDONLY=0, O_WRONLY=1, O_RDWR=2
        const creating = (flags & O.O_CREAT) !== 0;
        const truncating = (flags & O.O_TRUNC) !== 0;
        const exclusive = (flags & O.O_EXCL) !== 0;
        const appending = (flags & O.O_APPEND) !== 0;

        const exists = vfsLoad(path);

        if (exclusive && exists) {
            const err = new Error('EEXIST: file already exists, open \'' + path + '\'');
            err.code = 'EEXIST';
            callback(err);
            return;
        }
        if (!exists && !creating) {
            const err = new Error('ENOENT: no such file or directory, open \'' + path + '\'');
            err.code = 'ENOENT';
            callback(err);
            return;
        }
        if (!exists) {
            vfsFiles[path] = '';
        }
        if (truncating) {
            vfsFiles[path] = '';
        }

        const fd = vfsNextFD++;
        const content = vfsFiles[path] || '';
        const data = textEncoder.encode(content);
        vfsOpenFDs[fd] = {
            path: path,
            pos: appending ? data.length : 0,
            flags: flags,
            data: data,  // snapshot for reading
            written: [],  // accumulated write buffers
            dirty: false,
        };
        callback(null, fd);
    };

    // Override fs.close
    const origFsClose = globalThis.fs.close.bind(globalThis.fs);
    globalThis.fs.close = function(fd, callback) {
        const file = vfsOpenFDs[fd];
        if (!file) {
            origFsClose(fd, callback);
            return;
        }
        // If written, assemble final content and persist
        if (file.dirty) {
            // Reconstruct content from written chunks
            const totalLen = file.written.reduce((s, b) => s + b.length, 0);
            const merged = new Uint8Array(totalLen);
            let off = 0;
            for (const chunk of file.written) {
                merged.set(chunk, off);
                off += chunk.length;
            }
            vfsFiles[file.path] = textDecoder.decode(merged);
            vfsSave(file.path);
        }
        delete vfsOpenFDs[fd];
        callback(null);
    };

    // Override fs.stat (path-based)
    const origFsStat = globalThis.fs.stat.bind(globalThis.fs);
    globalThis.fs.stat = function(path, callback) {
        path = vfsNormalize(path);
        // "." is the virtual current directory
        if (path === '' || path === '.') {
            callback(null, vfsMakeStat('.', true));
            return;
        }
        if (vfsLoad(path)) {
            callback(null, vfsMakeStat(path, false));
            return;
        }
        origFsStat(path, callback);
    };

    // Override fs.mkdir - allow creating "." (it already exists)
    const origFsMkdir = globalThis.fs.mkdir.bind(globalThis.fs);
    globalThis.fs.mkdir = function(path, perm, callback) {
        path = vfsNormalize(path);
        if (path === '' || path === '.') {
            callback(null);
            return;
        }
        // For any other path, just succeed (flat filesystem)
        callback(null);
    };

    // Override fs.rename
    const origFsRename = globalThis.fs.rename.bind(globalThis.fs);
    globalThis.fs.rename = function(from, to, callback) {
        from = vfsNormalize(from);
        to = vfsNormalize(to);
        if (from in vfsFiles) {
            vfsFiles[to] = vfsFiles[from];
            vfsSave(to);
            vfsDelete(from);
            callback(null);
            return;
        }
        origFsRename(from, to, callback);
    };

    // Override fs.unlink
    const origFsUnlink = globalThis.fs.unlink.bind(globalThis.fs);
    globalThis.fs.unlink = function(path, callback) {
        path = vfsNormalize(path);
        if (path in vfsFiles) {
            vfsDelete(path);
            callback(null);
            return;
        }
        origFsUnlink(path, callback);
    };

    // ─── STDIN/STDOUT/STDERR Bridge ──────────────────────────────
    //
    // This lets x/term.Terminal (Go's line editor) work transparently:
    //   - It reads raw keystrokes from os.Stdin
    //   - It writes ANSI escape sequences to os.Stdout
    //   - The terminal emulator renders the escape sequences

    // Stdin buffer: terminal onData → stdinBuf; Go fs.read(0) ← stdinBuf
    let stdinBuf = new Uint8Array(0);
    let pendingRead = null;  // {buffer, offset, length, callback} when Go is waiting for input

    function concatBytes(a, b) {
        const c = new Uint8Array(a.length + b.length);
        c.set(a, 0);
        c.set(b, a.length);
        return c;
    }

    // Feed raw bytes from the terminal into the stdin buffer.
    // If Go is blocked on fs.read(0), immediately fulfill it.
    function feedStdin(data) {
        const encoded = textEncoder.encode(data);

        if (pendingRead) {
            const { buffer, offset, length, callback } = pendingRead;
            pendingRead = null;

            const n = Math.min(encoded.length, length);
            buffer.set(encoded.subarray(0, n), offset);

            // Buffer any leftover
            if (encoded.length > n) {
                stdinBuf = concatBytes(stdinBuf, encoded.subarray(n));
            }

            // Resume the blocked Go goroutine
            callback(null, n);
        } else {
            // No pending read, buffer for later
            stdinBuf = concatBytes(stdinBuf, encoded);
        }
    }

    // Save originals before overriding
    const origFsRead = globalThis.fs.read.bind(globalThis.fs);
    const origFsWrite = globalThis.fs.write.bind(globalThis.fs);
    const origFsWriteSync = globalThis.fs.writeSync.bind(globalThis.fs);

    // Override fs.fstat: make fd 0,1,2 report as character devices,
    // and support fstat on virtual file descriptors.
    const origFsFstat = globalThis.fs.fstat.bind(globalThis.fs);
    const S_IFCHR = 0o20000;
    const charDevMode = S_IFCHR | 0o620;
    globalThis.fs.fstat = function(fd, callback) {
        if (fd <= 2) {
            // Character device for stdin/stdout/stderr so
            // log.ConsoleLogging() returns true (checks ModeCharDevice).
            const ms = Date.now();
            callback(null, {
                dev: 0, ino: 0, mode: charDevMode, nlink: 1,
                uid: 0, gid: 0, rdev: 0, size: 0,
                blksize: 4096, blocks: 0,
                atimeMs: ms, mtimeMs: ms, ctimeMs: ms,
                isDirectory() { return false; },
            });
            return;
        }
        // Virtual file descriptors
        const file = vfsOpenFDs[fd];
        if (file) {
            callback(null, vfsMakeStat(file.path, false));
            return;
        }
        origFsFstat(fd, callback);
    };

    // Override fs.read: bridge fd=0 (stdin) to terminal input buffer,
    // and support reading from virtual file descriptors.
    globalThis.fs.read = function(fd, buffer, offset, length, position, callback) {
        // Virtual file descriptors
        const file = vfsOpenFDs[fd];
        if (file) {
            const readPos = (position !== null && position !== undefined) ? position : file.pos;
            const remaining = file.data.length - readPos;
            if (remaining <= 0) {
                callback(null, 0);  // EOF
                return;
            }
            const n = Math.min(remaining, length);
            buffer.set(file.data.subarray(readPos, readPos + n), offset);
            if (position === null || position === undefined) {
                file.pos += n;
            }
            callback(null, n);
            return;
        }

        if (fd !== 0) {
            origFsRead(fd, buffer, offset, length, position, callback);
            return;
        }

        // stdin
        if (stdinBuf.length > 0) {
            const n = Math.min(stdinBuf.length, length);
            buffer.set(stdinBuf.subarray(0, n), offset);
            stdinBuf = stdinBuf.subarray(n);
            if (stdinBuf.length === 0) stdinBuf = new Uint8Array(0);
            callback(null, n);
        } else {
            pendingRead = { buffer, offset, length, callback };
        }
    };

    // Override fs.writeSync: bridge fd=1,2 (stdout/stderr) to terminal
    globalThis.fs.writeSync = function(fd, buf) {
        if (fd === 1 || fd === 2) {
            // ghostty-web needs a buffer copy to avoid detached-buffer issues
            // after WASM memory growth; xterm.js can decode the view directly.
            const text = textDecoder.decode(copyBuffer ? new Uint8Array(buf) : buf);
            // x/term.Terminal already sends \r\n, but other Go code (log, fmt.Println)
            // may only send \n. We normalize: replace bare \n with \r\n.
            // This regex leaves existing \r\n unchanged.
            // Check for data: URL images in output — suppress the huge
            // base64 string and show a short colored placeholder instead.
            if (text.startsWith('data:image/')) {
                const nl = text.indexOf('\n');
                const dataUrl = nl >= 0 ? text.substring(0, nl) : text;
                document.getElementById('image').src = dataUrl;
                document.getElementById('image-container').style.display = 'block';
                onImageShow();
                // Show a brief colored marker instead of kilobytes of base64
                const kb = Math.round(dataUrl.length / 1024);
                const label = '\x1b[36m📷[image ' + kb + ' KB]\x1b[0m\n';
                const rest = nl >= 0 ? text.substring(nl) : '';
                const replacement = label + rest;
                const normalized = replacement.replace(/([^\r])\n/g, '$1\r\n').replace(/^\n/, '\r\n');
                termWrite(normalized);
                return buf.length;
            }

            const normalized = text.replace(/([^\r])\n/g, '$1\r\n').replace(/^\n/, '\r\n');
            termWrite(normalized);

            return buf.length;
        }
        return origFsWriteSync(fd, buf);
    };

    // Override fs.write (async version used by Go runtime)
    globalThis.fs.write = function(fd, buf, offset, length, position, callback) {
        if (fd === 1 || fd === 2) {
            if (offset !== 0 || length !== buf.length || position !== null) {
                callback(new Error("unsupported write parameters"), 0);
                return;
            }
            const n = this.writeSync(fd, buf);
            callback(null, n);
            return;
        }
        // Virtual file descriptors
        const file = vfsOpenFDs[fd];
        if (file) {
            const chunk = new Uint8Array(buf.buffer, buf.byteOffset + offset, length);
            file.written.push(chunk.slice());  // copy the data
            file.dirty = true;
            file.pos += length;
            callback(null, length);
            return;
        }
        origFsWrite(fd, buf, offset, length, position, callback);
    };

    // ─── Terminal Input → Go stdin ───────────────────────────────
    // onData sends raw keystrokes (including ANSI escape sequences
    // for arrow keys, etc.) — exactly what x/term.Terminal expects.
    // No JS-side line editing needed!
    xterm.onData(data => {
        feedStdin(data);
    });

    // ─── WASM Initialization ─────────────────────────────────────
    if (!WebAssembly.instantiateStreaming) {
        WebAssembly.instantiateStreaming = async (resp, importObject) => {
            const source = await (await resp).arrayBuffer();
            return await WebAssembly.instantiate(source, importObject);
        };
    }

    const go = new Go();
    // Set environment variables for the Go runtime.
    go.env = go.env || {};
    go.env["TERM"] = "xterm-256color";
    go.env["COLORTERM"] = "truecolor";

    WebAssembly.instantiateStreaming(fetch("/grol.wasm"), go.importObject).then((result) => {
        // Start Go runtime. main() registers JS functions then blocks on select{}.
        go.run(result.instance);

        // Update version display
        if (typeof grolVersion !== 'undefined') {
            document.getElementById('version').textContent = 'GROL ' + grolVersion + versionSuffix;
        }

        // Start the interactive REPL goroutine.
        // It will read from os.Stdin (bridged above) and write to os.Stdout.
        // Terminal globals are already set, so fortio.org/terminal.IsTerminal()
        // returns true and repl.Interactive() works.
        grolStartREPL(xterm.cols, xterm.rows);

        xterm.focus();

        // Save state (history + variables) when the page is being closed.
        // grolSaveState calls terminal.SaveHistory() and repl.AutoSave()
        // which write to the localStorage-backed virtual filesystem.
        window.addEventListener('pagehide', () => {
            if (typeof grolSaveState === 'function') {
                grolSaveState();
            }
        });
    }).catch((err) => {
        console.error('Failed to load WASM:', err);
        termWrite('\x1b[31mFailed to load GROL WASM: ' + err.message + '\x1b[0m\r\n');
    });
}
