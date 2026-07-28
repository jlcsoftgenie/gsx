const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const assert = require('node:assert/strict');
const oniguruma = require('../node_modules/vscode-oniguruma');
const textmate = require('../node_modules/vscode-textmate');

const extensionRoot = path.resolve(__dirname, '..');
const grammarPath = path.join(extensionRoot, 'syntaxes', 'gsx.tmLanguage.json');

async function loadGrammar() {
  const wasmPath = require.resolve('../node_modules/vscode-oniguruma/release/onig.wasm');
  await oniguruma.loadWASM(fs.readFileSync(wasmPath).buffer);
  const registry = new textmate.Registry({
    onigLib: Promise.resolve({
      createOnigScanner: (sources) => new oniguruma.OnigScanner(sources),
      createOnigString: (source) => new oniguruma.OnigString(source)
    }),
    loadGrammar: async (scopeName) => {
      if (scopeName !== 'source.gsx') {
        return null;
      }
      return textmate.parseRawGrammar(fs.readFileSync(grammarPath, 'utf8'), grammarPath);
    }
  });
  return registry.loadGrammar('source.gsx');
}

function tokenScopeAt(tokens, offset) {
  const token = tokens.find((candidate) => candidate.startIndex <= offset && offset < candidate.endIndex);
  return token?.scopes ?? [];
}

test('grammar keeps HTML tag scopes after self-closing expression attributes', async () => {
  const grammar = await loadGrammar();
  const lines = [
    'component UserCard(user User) {',
    '  <li class="user-card">',
    '    <img src={user.AvatarURL} alt={user.Name} />',
    '    <div class="content">',
    '      <h3>{user.Name}</h3>',
    '      <p>{user.Email}</p>',
    '    </div>',
    '  </li>',
    '}'
  ];
  let ruleStack = textmate.INITIAL;
  const tokenized = lines.map((line) => {
    const result = grammar.tokenizeLine(line, ruleStack);
    ruleStack = result.ruleStack;
    return result.tokens;
  });

  for (const [line, tag] of [[3, 'div'], [4, 'h3'], [5, 'p'], [6, 'div'], [7, 'li']]) {
    const offset = lines[line].lastIndexOf(tag);
    const scopes = tokenScopeAt(tokenized[line], offset);
    assert.ok(
      scopes.includes('entity.name.tag.gsx'),
      `expected ${tag} on line ${line + 1} to retain the HTML tag scope; got ${scopes.join(' ')}`
    );
  }
});

test('grammar closes a multiline self-closing tag before prose containing Go keywords', async () => {
  const grammar = await loadGrammar();
  const lines = [
    '<img',
    '  class="bio-portrait h-full w-full object-cover"',
    '  src="/assets/site/bio-speaking.png"',
    '  alt="Dr. Emily Stern speaking at a podium"',
    '/>',
    '</div>',
    '<div class="copy">',
    '  <p>Known for a warm, direct style.</p>',
    '</div>'
  ];
  let ruleStack = textmate.INITIAL;
  const tokenized = lines.map((line) => {
    const result = grammar.tokenizeLine(line, ruleStack);
    ruleStack = result.ruleStack;
    return result.tokens;
  });

  for (const [line, tag] of [[5, 'div'], [6, 'div'], [7, 'p'], [8, 'div']]) {
    const offset = lines[line].lastIndexOf(tag);
    const scopes = tokenScopeAt(tokenized[line], offset);
    assert.ok(
      scopes.includes('entity.name.tag.gsx'),
      `expected ${tag} on line ${line + 1} to retain the HTML tag scope; got ${scopes.join(' ')}`
    );
  }

  const proseScopes = tokenScopeAt(tokenized[7], lines[7].indexOf('for'));
  assert.ok(
    !proseScopes.includes('meta.control.flow.gsx'),
    `expected prose to stay outside a control-flow scope; got ${proseScopes.join(' ')}`
  );
});
