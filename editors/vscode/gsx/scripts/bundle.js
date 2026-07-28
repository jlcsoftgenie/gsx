const esbuild = require('esbuild');

const common = {
  bundle: true,
  platform: 'node',
  target: 'node20',
  format: 'cjs',
  sourcemap: false,
  logLevel: 'info'
};

Promise.all([
  esbuild.build({
    ...common,
    entryPoints: ['src/client.ts'],
    outfile: 'dist/client.js',
    external: ['vscode']
  }),
  esbuild.build({
    ...common,
    entryPoints: ['src/server.ts'],
    outfile: 'dist/server.js'
  })
]).catch((err) => {
  console.error(err);
  process.exit(1);
});
