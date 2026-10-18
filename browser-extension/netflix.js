// netflix.js — Netflix doesn't fill in the Media Session metadata, so the
// browser reports every show as just "Netflix". This reads the show and
// episode from the player's title overlay and sets it; the browser passes it
// on (MPRIS on Linux) to PALT and the desktop's media controls.
(() => {
  let last = '';
  let page = '';
  let meta = null;

  const read = () => {
    const box = document.querySelector('[data-uia="video-title"]');
    if (!box) return null;
    const show = box.querySelector('h4')?.textContent?.trim() || '';
    const parts = [...box.querySelectorAll('span')].map((s) => s.textContent.trim()).filter(Boolean);
    if (!show && !parts.length) return null;
    // Series: "Show" + "E3" + "Episode name"; movie: just the title.
    return parts.length ? { title: parts.join(' · '), artist: show } : { title: show || box.textContent.trim(), artist: '' };
  };

  const tick = () => {
    if (!('mediaSession' in navigator)) return;
    if (location.pathname !== page) {
      // New title: forget the old one until the overlay shows the new one.
      page = location.pathname;
      meta = null;
      last = '';
    }
    if (!page.startsWith('/watch')) return;
    meta = read() || meta;
    if (!meta) return;
    const key = meta.title + '|' + meta.artist;
    if (key === last && navigator.mediaSession.metadata) return;
    last = key;
    const art = document.querySelector('meta[property="og:image"]')?.content;
    navigator.mediaSession.metadata = new MediaMetadata({
      title: meta.title,
      artist: meta.artist,
      album: 'Netflix',
      artwork: art ? [{ src: art }] : [],
    });
  };

  setInterval(tick, 2000);
})();
