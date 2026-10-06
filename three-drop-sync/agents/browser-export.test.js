const { test } = require('node:test');
const assert = require('node:assert/strict');
const { threeDropReference, threeDropVisiblePage, exportThreeDrop } = require('./browser-export.js');

const base = 'https://three-drop.com';
const options = { savedUrl: `${base}/favorites/saved`, likesUrl: `${base}/favorites/likes`, collectionsUrl: `${base}/favorites/collections` };
const model = id => ({ website: 'printables', title: `Synthetic model ${id}`, sourceUrl: `https://www.printables.com/model/${id}-example`, liked: true });
const page = (heading, count, models = [], collections = [], next = null) => ({ heading, count, models, collections, next });

function mockTab(pages, collectionTargets = [], blockedURL = null) {
  let url = options.savedUrl;
  const calls = [];
  const tab = {
    calls,
    async goto(next) { calls.push(next); if (next === blockedURL) throw new Error('Browser safety block'); url = next; },
    async url() { return url; },
    playwright: {
      locator() { return { async waitFor() {}, async click() { await tab.goto(new URL(pages[url].next, url).href); } }; },
      async domSnapshot() {},
      async evaluate() { return structuredClone(pages[url]); },
      getByRole() {
        return { async all() {
          return [{ async evaluate() { return 'Filters'; } }, ...pages[url].collections.map((row, index) => ({
            async evaluate() { return row.name; }, async click() { await tab.goto(collectionTargets[index]); },
          }))];
        } };
      },
    },
  };
  return tab;
}

test('provider IDs are parsed only from verified routes and origins', () => {
  assert.equal(threeDropReference('https://makerworld.com/en/models/42-example', 'makerworld').externalId, '42');
  assert.equal(threeDropReference('https://www.thingiverse.com/thing:42', 'thingiverse').externalId, '42');
  for (const [url, site] of [
    ['https://evil.example/model/42', 'printables'], ['https://printables.com/model/42?token=secret', 'printables'],
    ['https://printables.com/model/42#secret', 'printables'], ['https://printables.com/model/0', 'printables'],
    ['https://cults3d.com/en/3d-model/art/example', 'cults'],
  ]) assert.throws(() => threeDropReference(url, site));
});

test('standalone heading count is read despite adjacent collection controls', () => {
  const previousDocument = global.document;
  const previousStyle = global.getComputedStyle;
  const element = text => ({ innerText: text, getClientRects: () => [{}] });
  const count = element('( 12 )');
  const h1 = element('Collections');
  h1.parentElement = {
    ...element('Collections ( 12 ) Merge duplicate collections Select New Collection'),
    querySelector: () => null,
    querySelectorAll: () => [h1, count, element('Merge duplicate collections'), element('Select'), element('New Collection')],
  };
  global.document = { querySelectorAll: selector => selector === 'main h1' ? [h1] : [] };
  global.getComputedStyle = () => ({ visibility: 'visible' });
  try {
    assert.equal(threeDropVisiblePage().count, 12);
    h1.parentElement.querySelectorAll = () => [h1, count, element('( 13 )')];
    assert.throws(() => threeDropVisiblePage(), /ambiguous/);
  } finally {
    global.document = previousDocument;
    global.getComputedStyle = previousStyle;
  }
});

test('fresh headings preserve distinct collections with duplicate names and full membership union', async () => {
  const first = `${base}/favorites/collections/11111111-1111-1111-1111-111111111111`;
  const second = `${base}/favorites/collections/22222222-2222-2222-2222-222222222222`;
  const unliked = { ...model(3), liked: false };
  const pages = {
    [options.savedUrl]: page('All Saved Models', 3),
    [options.likesUrl]: page('Liked Models', 2, [model(1)], [], '/favorites/likes?page=2'),
    [`${options.likesUrl}?page=2`]: page('Liked Models', 2, [model(2)]),
    [options.collectionsUrl]: page('Collections', 2, [], [{ name: 'Same name', itemCount: 1 }, { name: 'Same name', itemCount: 2 }]),
    [first]: page('Same name', 1, [model(1)]),
    [second]: page('Same name', 2, [model(1), unliked]),
  };
  const { manifest, error } = await exportThreeDrop(mockTab(pages, [first, second]), options);
  assert.equal(error, null);
  assert.equal(manifest.complete, true);
  assert.equal(manifest.models.length, 3);
  assert.equal(manifest.collections.length, 2);
  assert.equal(manifest.models[0].collectionIds.length, 2);
  assert.equal(manifest.models[2].liked, false);
});

test('safety block stops immediately and leaves incomplete result', async () => {
  const next = `${options.likesUrl}?page=2`;
  const tab = mockTab({
    [options.savedUrl]: page('All Saved Models', 2),
    [options.likesUrl]: page('Liked Models', 2, [model(1)], [], '/favorites/likes?page=2'),
  }, [], next);
  const { manifest, error } = await exportThreeDrop(tab, options);
  assert.equal(manifest.complete, false);
  assert.match(error.message, /Browser safety block/);
  assert.equal(tab.calls.filter(url => url === next).length, 1);
  assert.equal(manifest.models.length, 1);
});

test('missing final page and unknown provider never yield a complete manifest', async () => {
  for (const cards of [[model(1)], [{ ...model(1), website: 'cults' }]]) {
    const tab = mockTab({
      [options.savedUrl]: page('All Saved Models', 2),
      [options.likesUrl]: page('Liked Models', 2, cards),
    });
    const { manifest, error } = await exportThreeDrop(tab, options);
    assert.equal(manifest.complete, false);
    assert.ok(error);
  }
});
