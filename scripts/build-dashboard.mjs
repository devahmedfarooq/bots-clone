import { copyFile } from 'node:fs/promises';

await copyFile('node_modules/htmx.org/dist/htmx.min.js', 'internal/service/web/htmx.min.js');
await copyFile('node_modules/htmx.org/LICENSE', 'internal/service/web/htmx.LICENSE');
