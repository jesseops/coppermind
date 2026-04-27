// app.js — Alpine.js component registrations and global utilities for Coppermind.

document.addEventListener('alpine:init', function () {

    // ── Theme store ──
    Alpine.store('theme', {
        current: localStorage.getItem('coppermind-theme') || 'auto',
        set(name) {
            this.current = name;
            document.documentElement.setAttribute('data-theme', name);
            localStorage.setItem('coppermind-theme', name);
        }
    });

    // ── Confirm delete component ──
    Alpine.data('confirmDelete', function () {
        return {
            confirmAndSubmit(message, event) {
                if (!confirm(message)) {
                    event.preventDefault();
                }
            }
        };
    });

    // ── Chapter select component (reader) ──
    Alpine.data('chapterSelect', function () {
        return {
            navigate(editionID, event) {
                window.location.href = '/read/' + editionID + '?chapter=' + event.target.value;
            }
        };
    });

    // ── Edition edit toggle component ──
    Alpine.data('editionEdit', function () {
        return {
            editing: false,
            label: '✏️ Edit',
            toggle() {
                this.editing = !this.editing;
                this.label = this.editing ? '✕ Close' : '✏️ Edit';
            }
        };
    });

    // ── Reading progress tracker (reader page) ──
    Alpine.data('readerProgress', function (editionID, chapter, totalChapters, workID, csrfToken) {
        return {
            init() {
                var self = this;
                self.saveProgress();
                setInterval(function () { self.saveProgress(); }, 60000);
            },
            saveProgress() {
                var progress = totalChapters > 0 ? chapter / totalChapters : 0;
                var body = 'progress=' + progress + '&chapter_index=' + chapter;
                var x = new XMLHttpRequest();
                x.open('POST', '/api/v1/me/reading/' + editionID, true);
                x.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
                x.setRequestHeader('X-CSRF-Token', csrfToken);
                x.send(body);
            },
            markFinished() {
                var body = 'progress=1&chapter_index=' + chapter + '&status=finished';
                var x = new XMLHttpRequest();
                x.open('POST', '/api/v1/me/reading/' + editionID, true);
                x.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
                x.setRequestHeader('X-CSRF-Token', csrfToken);
                x.onload = function () {
                    alert('Marked as finished!');
                    window.location.href = '/works/' + workID;
                };
                x.send(body);
            }
        };
    });

    // ── Audio player progress tracker ──
    Alpine.data('audioPlayer', function (editionID, totalTracks, csrfToken) {
        return {
            currentTrack: 0,
            init() {
                var self = this;
                this.$root.querySelectorAll('audio').forEach(function (el) {
                    el.addEventListener('play', function () {
                        self.currentTrack = parseInt(el.getAttribute('data-track-index')) || 0;
                        self.saveProgress(self.currentTrack, 0);
                    });
                    el.addEventListener('pause', function () {
                        self.saveProgress(self.currentTrack, Math.floor(el.currentTime));
                    });
                    el.addEventListener('ended', function () {
                        self.saveProgress(self.currentTrack, Math.floor(el.duration));
                    });
                });
            },
            saveProgress(trackIndex, seconds) {
                var progress = totalTracks > 0 ? trackIndex / totalTracks : 0;
                var body = 'progress=' + progress + '&chapter_index=' + trackIndex;
                if (seconds) body += '&position_seconds=' + seconds;
                var x = new XMLHttpRequest();
                x.open('POST', '/api/v1/me/reading/' + editionID, true);
                x.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
                x.setRequestHeader('X-CSRF-Token', csrfToken);
                x.send(body);
            }
        };
    });

    // ── Receive page (e-reader key generation + polling) ──
    Alpine.data('receiveEReader', function () {
        return {
            currentKey: null,
            pollTimer: null,
            keyDisplay: '····',
            statusMsg: 'Generating code...',
            downloadReady: false,
            downloadName: '',
            downloadHref: '#',

            init() {
                this.generateKey();
            },

            generateKey() {
                var self = this;
                if (self.pollTimer) clearInterval(self.pollTimer);
                self.keyDisplay = '····';
                self.downloadReady = false;
                self.statusMsg = 'Generating code...';

                var x = new XMLHttpRequest();
                x.open('POST', '/receive/generate', true);
                x.onload = function () {
                    if (x.status === 200 && x.responseText.length > 0) {
                        self.currentKey = x.responseText;
                        self.keyDisplay = self.currentKey;
                        self.statusMsg = 'Waiting for a book to be sent...';
                        if (self.pollTimer) clearInterval(self.pollTimer);
                        self.pollTimer = setInterval(function () { self.pollStatus(); }, 4000);
                    } else {
                        self.statusMsg = 'Error generating code. Try again.';
                    }
                };
                x.onerror = function () {
                    self.statusMsg = 'Connection error. Check your wifi.';
                };
                x.send(null);
            },

            pollStatus() {
                var self = this;
                if (!self.currentKey) return;
                var x = new XMLHttpRequest();
                x.open('GET', '/receive/status/' + self.currentKey, true);
                x.onload = function () {
                    if (x.status === 404) {
                        self.currentKey = null;
                        if (self.pollTimer) clearInterval(self.pollTimer);
                        self.keyDisplay = '····';
                        self.downloadReady = false;
                        self.statusMsg = 'Code expired. Generating new one...';
                        setTimeout(function () { self.generateKey(); }, 1000);
                        return;
                    }
                    if (x.status !== 200) return;
                    try {
                        var data = JSON.parse(x.responseText);
                        if (data.file) {
                            self.downloadReady = true;
                            self.downloadName = '📖 ' + data.file.name;
                            self.downloadHref = '/receive/download/' + self.currentKey + '/' + encodeURIComponent(data.file.name);
                            self.statusMsg = 'Tap the button above to download.';
                        } else {
                            self.downloadReady = false;
                            self.statusMsg = 'Waiting for a book to be sent...';
                        }
                    } catch (e) { }
                };
                x.send(null);
            }
        };
    });

    // ── Receive page URL display (non e-reader) ──
    Alpine.data('receiveUrl', function () {
        return {
            url: '',
            init() {
                this.url = window.location.origin + '/receive';
            }
        };
    });

    // ── Work page: auto-refresh on cover/metadata changes ──
    Alpine.data('workPage', function () {
        return {
            init() {
                document.body.addEventListener('coverUpdated', function () {
                    location.reload();
                });
                document.body.addEventListener('metadataUpdated', function () {
                    location.reload();
                });
            }
        };
    });
});

// ── Apply theme before first paint (runs immediately, before Alpine) ──
(function () {
    var t = localStorage.getItem('coppermind-theme') || 'auto';
    document.documentElement.setAttribute('data-theme', t);
})();

// ── Card action menus: single delegated handler instead of per-card Alpine ──
(function () {
    document.addEventListener('click', function (e) {
        var btn = e.target.closest('.card-actions-btn');
        if (btn) {
            e.preventDefault();
            e.stopPropagation();
            var card = btn.closest('.card-actions');
            var wasOpen = card.classList.contains('open');
            // Close all open menus first.
            document.querySelectorAll('.card-actions.open').forEach(function (el) {
                el.classList.remove('open');
            });
            if (!wasOpen) card.classList.add('open');
            return;
        }
        // Clicks inside the menu (forms, inputs) stay open.
        if (e.target.closest('.card-actions-menu')) return;
        // Everything else closes open menus.
        document.querySelectorAll('.card-actions.open').forEach(function (el) {
            el.classList.remove('open');
        });
    });
})();

// ── Lazy-load images using IntersectionObserver ──
(function () {
    function loadImage(img) {
        var src = img.getAttribute('data-src');
        if (src) {
            img.src = src;
            img.removeAttribute('data-src');
            img.classList.remove('lazy');
        }
    }

    function initLazyImages(root) {
        var images = (root || document).querySelectorAll('img.lazy[data-src]');
        if (!images.length) return;

        if ('IntersectionObserver' in window) {
            var observer = new IntersectionObserver(function (entries) {
                entries.forEach(function (entry) {
                    if (entry.isIntersecting) {
                        loadImage(entry.target);
                        observer.unobserve(entry.target);
                    }
                });
            }, { rootMargin: '200px' });
            images.forEach(function (img) { observer.observe(img); });
        } else {
            // Fallback: load all immediately.
            images.forEach(loadImage);
        }
    }

    // Initial page load.
    initLazyImages();

    // Re-init after HTMX swaps new content in.
    document.body.addEventListener('htmx:afterSwap', function (e) {
        initLazyImages(e.detail.target);
    });
})();
