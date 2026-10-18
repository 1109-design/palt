// netflix.js — Netflix doesn't fill in the Media Session metadata, so the
// browser reports every show as just "Netflix". This reads the show and
// episode from the player's title overlay and sets it; the browser passes it
// on (MPRIS on Linux) to PALT and the desktop's media controls.
//
// Browsers shrink artwork to a thumbnail (~150px) before passing it on, so
// the full-size picture's URL also rides along in the album as
// "Netflix ⟨art:URL⟩"; PALT (desktop/mpris_linux.go) takes it from there.
(() => {
  let last = '';
  let page = '';
  let meta = null;

  // Browsers keep the previous page's artwork (say, a YouTube thumbnail) when
  // new metadata has none, so there is always a picture: the show's, else a
  // Netflix "N" tile drawn here.
  let tile = '';
  const fallbackArt = () => {
    if (tile) return tile;
    const c = document.createElement('canvas');
    c.width = c.height = 512;
    const g = c.getContext('2d');
    g.fillStyle = '#141414';
    g.fillRect(0, 0, 512, 512);
    g.fillStyle = '#E50914';
    g.font = 'bold 380px Arial, sans-serif';
    g.textAlign = 'center';
    g.textBaseline = 'middle';
    g.fillText('N', 256, 280);
    tile = c.toDataURL('image/png');
    return tile;
  };

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
      album: art ? `Netflix ⟨art:${art}⟩` : 'Netflix',
      artwork: art ? [{ src: art }] : [{ src: fallbackArt(), sizes: '512x512', type: 'image/png' }],
    });
  };

  setInterval(tick, 2000);
})();
