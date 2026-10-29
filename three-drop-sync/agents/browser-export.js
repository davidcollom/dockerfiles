/* DOM-only helper for the supported cloud-browser tab.playwright API.
 * Load this source into cua_repl, then call exportThreeDrop(tab, options).
 * No fetch, browser state, credentials, cookies or storage are read.
 */

function threeDropReference(sourceUrl, website) {
  const url = new URL(sourceUrl);
  if (url.protocol !== 'https:' || url.username || url.password || url.port || url.search || url.hash) {
    throw new Error('Unsupported source URL');
  }
  const routes = {
    printables: { host: 'printables.com', path: /^\/model\/([1-9]\d*)(?:-[^/]+)?\/?$/ },
    thingiverse: { host: 'thingiverse.com', path: /^\/thing:([1-9]\d*)\/?$/ },
    makerworld: { host: 'makerworld.com', path: /^\/(?:[a-z]{2}\/)?models\/([1-9]\d*)(?:-[^/]+)?\/?$/ },
  };
  const route = routes[website];
  const match = route && url.pathname.match(route.path);
  if (!match || ![route.host, `www.${route.host}`].includes(url.hostname)) {
    throw new Error('Provider URL route is not verified');
  }
  return { website, externalId: match[1], sourceUrl: url.href };
}

// Runs inside the rendered document, not against application state or network.
function threeDropVisiblePage() {
  const visible = el => !!el && el.getClientRects().length > 0 && getComputedStyle(el).visibility !== 'hidden';
  const headings = [...document.querySelectorAll('main h1')].filter(visible);
  if (headings.length !== 1) throw new Error('Expected one visible account heading');
  const h1 = headings[0];
  let count;
  // Counts are rendered immediately around the page heading. Do not scan cards.
  for (let parent = h1.parentElement, depth = 0; parent && depth < 3; parent = parent.parentElement, depth++) {
    if (parent.querySelector('h3')) break;
    const values = new Set();
    for (const element of [parent, ...parent.querySelectorAll('*')].filter(visible)) {
      const match = element.innerText?.trim().match(/^\(\s*([\d,]+)\s*\)$/);
      if (match) values.add(Number(match[1].replaceAll(',', '')));
    }
    if (values.size > 1) throw new Error('Account heading count is ambiguous');
    if (values.size === 1) { count = [...values][0]; break; }
  }
  if (!Number.isSafeInteger(count) || count < 0) throw new Error('Account count is not visible');
  const models = [], collections = [];
  for (const heading of [...document.querySelectorAll('main h3')].filter(visible)) {
    const title = heading.innerText.trim();
    if (title === 'Filters') continue;
    const card = heading.parentElement?.parentElement;
    if (!card) throw new Error('Card structure changed');
    const source = [...card.querySelectorAll('a[href]')].find(a => visible(a) && a.getAttribute('aria-label') === title && a.href.startsWith('https://'));
    if (source) {
      const icon = [...card.querySelectorAll('img')].find(img => /\/assets\/images\/website\//.test(img.getAttribute('src') || ''));
      const website = icon?.getAttribute('alt');
      const unlike = [...card.querySelectorAll('button')].some(button => button.getAttribute('aria-label') === 'Unlike');
      const like = [...card.querySelectorAll('button')].some(button => button.getAttribute('aria-label') === 'Like');
      if (!website || !title || (!like && !unlike)) throw new Error('Model card metadata is incomplete');
      models.push({ website, title, sourceUrl: source.href, liked: unlike });
    } else {
      let root = heading.parentElement, itemCount;
      for (let depth = 0; root && depth < 3; depth++, root = root.parentElement) {
        const paragraphs = [...root.querySelectorAll('p')].filter(visible);
        const match = paragraphs.map(p => p.innerText.trim().match(/^([\d,]+)\s+items?$/)).find(Boolean);
        if (match) { itemCount = Number(match[1].replaceAll(',', '')); break; }
      }
      if (!Number.isSafeInteger(itemCount)) throw new Error('Collection item count is missing');
      collections.push({ name: title, itemCount });
    }
  }
  const next = [...document.querySelectorAll('a[aria-label="Next page"]')].filter(visible);
  if (next.length > 1) throw new Error('Ambiguous pagination');
  return { heading: h1.innerText.trim(), count, models, collections, next: next[0]?.getAttribute('href') || null };
}

async function exportThreeDrop(tab, { likesUrl, collectionsUrl, savedUrl, maxPages = 1000 } = {}) {
  const manifest = {
    schemaVersion: 1, source: 'three-drop', capturedAt: new Date().toISOString(), complete: false,
    totals: { likes: 0, collections: 0, savedModels: 0 }, models: [], collections: [],
  };
  const inventory = new Map();
  const accountURL = raw => {
    const url = new URL(raw, 'https://three-drop.com');
    if (url.origin !== 'https://three-drop.com' || !url.pathname.startsWith('/favorites') || url.username || url.password || url.hash) {
      throw new Error('Expected an observed 3Drop favourites URL');
    }
    return url.href;
  };
  const read = async () => {
    await tab.playwright.locator('main h1').waitFor({ state: 'visible', timeoutMs: 15000 });
    await tab.playwright.domSnapshot();
    return tab.playwright.evaluate(threeDropVisiblePage);
  };
  const navigate = async url => { await tab.goto(accountURL(url)); return read(); };
  const merge = (card, collectionId) => {
    const reference = threeDropReference(card.sourceUrl, card.website);
    const key = `${reference.website}:${reference.externalId}`;
    let model = inventory.get(key);
    if (!model) {
      model = { ...reference, title: card.title, liked: card.liked, collectionIds: [] };
      inventory.set(key, model);
    } else if (model.liked !== card.liked) throw new Error('Like state changed during export');
    if (collectionId && !model.collectionIds.includes(collectionId)) model.collectionIds.push(collectionId);
    return key;
  };
  const pages = async (first, expectedHeading, collectionId) => {
    let page = first, visited = new Set(), references = new Set(), count = first.count;
    for (let n = 0; n < maxPages; n++) {
      const current = accountURL(await tab.url());
      if (visited.has(current)) throw new Error('Pagination loop');
      visited.add(current);
      if (page.heading !== expectedHeading || page.count !== count || page.collections.length) throw new Error('Page changed during export');
      for (const card of page.models) {
        const key = merge(card, collectionId);
        if (references.has(key)) throw new Error('Duplicate model across pages');
        references.add(key);
      }
      if (!page.next) {
        if (references.size !== count) throw new Error('Rendered inventory count does not match');
        return references;
      }
      // Follow the fresh rendered link, without guessing the next page number.
      const nextURL = new URL(page.next, current);
      if (nextURL.pathname !== new URL(current).pathname) throw new Error('Pagination changed route');
      await tab.playwright.locator('a[aria-label="Next page"]').click();
      page = await read();
      if (accountURL(await tab.url()) !== accountURL(nextURL.href)) throw new Error('Pagination did not reach the observed target');
    }
    throw new Error('Page limit reached');
  };
  try {
    if (!likesUrl || !collectionsUrl || !savedUrl) throw new Error('Provide all three observed account URLs');
    const saved = await navigate(savedUrl);
    if (saved.heading !== 'All Saved Models') throw new Error('Expected saved-model account page');
    manifest.totals.savedModels = saved.count;
    const likedPage = await navigate(likesUrl);
    if (likedPage.heading !== 'Liked Models') throw new Error('Expected liked-model account page');
    manifest.totals.likes = likedPage.count;
    await pages(likedPage, 'Liked Models');
    let index = await navigate(collectionsUrl);
    if (index.heading !== 'Collections' || index.models.length || index.next) throw new Error('Unsupported collections index');
    manifest.totals.collections = index.count;
    if (index.collections.length !== index.count) throw new Error('Collection index count does not match');
    const rows = index.collections;
    for (let i = 0; i < rows.length; i++) {
      if (i > 0) index = await navigate(collectionsUrl);
      if (JSON.stringify(index.collections) !== JSON.stringify(rows)) throw new Error('Collections changed during export');
      // Refresh .all() after returning to the list. Position disambiguates duplicate names.
      const headings = await tab.playwright.getByRole('heading', { level: 3 }).all();
      const matching = [];
      for (const heading of headings) {
        const text = await heading.evaluate(el => el.innerText.trim());
        if (text !== 'Filters') matching.push({ heading, text });
      }
      if (matching.length !== rows.length || matching[i].text !== rows[i].name) throw new Error('Collection heading order changed');
      await matching[i].heading.click();
      const detail = await read();
      const actual = new URL(accountURL(await tab.url()));
      const match = actual.pathname.match(/^\/favorites\/collections\/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\/?$/i);
      if (!match || detail.heading !== rows[i].name || detail.count !== rows[i].itemCount) throw new Error('Collection identity or count mismatch');
      if (manifest.collections.some(collection => collection.id === match[1])) throw new Error('Duplicate collection ID');
      manifest.collections.push({ id: match[1], ...rows[i] });
      await pages(detail, rows[i].name, match[1]);
    }
    manifest.models = [...inventory.values()];
    if (manifest.models.length !== manifest.totals.savedModels || manifest.models.filter(model => model.liked).length !== manifest.totals.likes) {
      throw new Error('Account totals do not match the captured union');
    }
    // Recheck the counts after capture; a changing account must not publish as complete.
    for (const [url, count, heading] of [[savedUrl, manifest.totals.savedModels, 'All Saved Models'], [likesUrl, manifest.totals.likes, 'Liked Models'], [collectionsUrl, manifest.totals.collections, 'Collections']]) {
      const page = await navigate(url);
      if (page.count !== count || page.heading !== heading) throw new Error('Account changed during export');
    }
    manifest.complete = true;
    return { manifest, error: null };
  } catch (error) {
    manifest.models = [...inventory.values()];
    // Preserve the partial result locally. Caller must stop; never retry a browser safety block.
    return { manifest, error };
  }
}

if (typeof module !== 'undefined') module.exports = { threeDropReference, threeDropVisiblePage, exportThreeDrop };
