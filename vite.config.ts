import fs from 'fs';
import { resolve } from 'path';

import { type UserConfig, type Plugin, defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue';
import Checker from 'vite-plugin-checker';
import { minify } from 'terser';
import { execFileSync } from 'node:child_process';

import packageFile from './package.json';
import contributorsFile from './contributors.json';
import thirdPartyLicenseFile from './third-party-dependencies.json';

const SRC_DIR = resolve(__dirname, './src');
const PUBLIC_DIR = resolve(__dirname, './public');
const BUILD_DIR = resolve(__dirname, './dist',);

function minifyWorkerWithTerser(): Plugin {
    const minifyOptionsHash = `terser-${packageFile.devDependencies.terser}-compress-mangle-no-comments`;

    return {
        name: 'minify-worker-with-terser',
        apply: 'build',
        augmentChunkHash(): string {
            return minifyOptionsHash;
        },
        generateBundle(_, bundle): Promise<void> {
            const minifyPromises: Promise<void>[] = [];

            for (const output of Object.values(bundle)) {
                if (output.type !== 'chunk') {
                    continue;
                }

                minifyPromises.push(minify(output.code, {
                    compress: true,
                    format: {
                        comments: false
                    },
                    mangle: true
                }).then(result => {
                    if (!result.code) {
                        throw new Error(`Terser failed to minify worker ${output.fileName}.`);
                    }

                    output.code = result.code;
                    output.map = null;
                }));
            }

            return Promise.all(minifyPromises).then(() => undefined);
        }
    };
}

function injectFramework7CssFile({ htmlFileName, placeHolders }: { htmlFileName: string, placeHolders: { name: string, srcFileName: string, distFileNamePrefix: string }[] }): Plugin[] {
    return [
        {
            name: 'inject-framework7-css-file:serve',
            apply: 'serve',
            enforce: 'post',
            transformIndexHtml(html: string): string {
                for (const placeholder of placeHolders) {
                    html = html.replace(`{{${placeholder.name}}}`, `${placeholder.srcFileName}`);
                }
                return html;
            }
        },
        {
            name: 'inject-framework7-css-file:build',
            apply: 'build',
            enforce: 'post',
            generateBundle(_, bundle): void {
                const placeholderCssFilePathMap: Record<string, string> = {};

                for (const fileName of Object.keys(bundle)) {
                    for (const placeholder of placeHolders) {
                        if (fileName.startsWith(placeholder.distFileNamePrefix)) {
                            placeholderCssFilePathMap[placeholder.name] = fileName;
                            break;
                        }
                    }
                }

                const htmlAsset = bundle[htmlFileName];

                if (!htmlAsset || htmlAsset.type !== 'asset') {
                    return;
                }

                let html = htmlAsset.source as string;

                for (const [placeholder, filePath] of Object.entries(placeholderCssFilePathMap)) {
                    html = html.replace(`{{${placeholder}}}`, `./${filePath}`);
                }

                htmlAsset.source = html;
            }
        }
    ];
}

function offlineOCR():Plugin {
    const files:Record<string,string>={'worker.min.js':'node_modules/tesseract.js/dist/worker.min.js'};
    for(const suffix of ['', '-simd', '-lstm', '-simd-lstm']) files[`tesseract-core${suffix}.wasm.js`]=`node_modules/tesseract.js-core/tesseract-core${suffix}.wasm.js`;
    for(const lang of ['chi_sim','eng']) files[`${lang}.traineddata.gz`]=`node_modules/@tesseract.js-data/${lang}/4.0.0_best_int/${lang}.traineddata.gz`;
    return {name:'offline-bill-ocr',generateBundle(){for(const [name,path] of Object.entries(files))this.emitFile({type:'asset',fileName:`js/ocr/${name}`,source:fs.readFileSync(resolve(__dirname,path))});},configureServer(server){server.middlewares.use('/js/ocr/',(req,res,next)=>{const path=files[(req.url||'').split('?')[0]!];if(!path){next();return;}res.setHeader('Content-Type',path.endsWith('.js')?'application/javascript':'application/octet-stream');fs.createReadStream(resolve(__dirname,path)).pipe(res);});}};
}

export default defineConfig(() => {
    const licenseContent = ['./LICENSE', './NOTICE', './licenses/ezbookkeeping-MIT-LICENSE']
        .map(path => fs.readFileSync(path, { encoding: 'utf-8' })).join('\n\n');
    const buildUnixTime = process.env['buildUnixTime'] || '';

    const options: UserConfig = {
        root: SRC_DIR,
        publicDir: PUBLIC_DIR,
        base: './',
        define: {
            __EZBOOKKEEPING_IS_PRODUCTION__: process.env['NODE_ENV'] === 'production',
            __EZBOOKKEEPING_VERSION__: JSON.stringify(packageFile.version),
            __EZBOOKKEEPING_BUILD_UNIX_TIME__: JSON.stringify(buildUnixTime),
            __EZBOOKKEEPING_BUILD_COMMIT_HASH__: JSON.stringify(execFileSync('git', ['rev-parse', '--short=7', 'HEAD'], { cwd: __dirname, encoding: 'utf8' }).trim()),
            __EZBOOKKEEPING_CONTRIBUTORS__: JSON.stringify(contributorsFile),
            __EZBOOKKEEPING_LICENSE__: JSON.stringify(licenseContent),
            __EZBOOKKEEPING_THIRD_PARTY_LICENSES__: JSON.stringify(thirdPartyLicenseFile)
        },
        plugins: [
            offlineOCR(),
            vue({
                template: {
                    compilerOptions: {
                        isCustomElement: tag => tag.includes('swiper-')
                    }
                }
            }),
            injectFramework7CssFile({
                htmlFileName: 'mobile.html',
                placeHolders: [
                    {
                        name: 'framework7-ltr-css-filepath',
                        srcFileName: 'mobile-ltr.scss',
                        distFileNamePrefix: 'css/vendor-framework7-ltr'
                    },
                    {
                        name: 'framework7-rtl-css-filepath',
                        srcFileName: 'mobile-rtl.scss',
                        distFileNamePrefix: 'css/vendor-framework7-rtl'
                    }
                ]
            }),
            Checker({
                vueTsc: true
            }),

        ],
        worker: {
            plugins: () => [
                minifyWorkerWithTerser()
            ],
            rolldownOptions: {
                output: {
                    chunkFileNames: 'js/[name]-[hash].js',
                    entryFileNames: 'js/[name]-[hash].js'
                }
            }
        },
        build: {
            target: [
                'chrome119'
            ],
            minify: 'terser',
            outDir: BUILD_DIR,
            sourcemap: false,
            assetsInlineLimit: 0,
            emptyOutDir: true,
            rolldownOptions: {
                input: {
                    index: resolve(SRC_DIR, 'index.html'),
                    mobile: resolve(SRC_DIR, 'mobile.html'),
                    'vendor-framework7-ltr': resolve(SRC_DIR, 'mobile-ltr.scss'),
                    'vendor-framework7-rtl': resolve(SRC_DIR, 'mobile-rtl.scss')
                },
                output: {
                    assetFileNames: assetInfo => {
                        const fileExt = assetInfo.names[0]?.split('.')[1];

                        if (!fileExt) {
                            throw new Error('Invalid asset file name.');
                        }

                        let assetType = fileExt;

                        if (/png|jpe?g|gif|tiff|bmp|ico/i.test(fileExt)) {
                            assetType = 'img';
                        } else if (/ttf|woff|woff2|svg|eot/i.test(fileExt)) {
                            assetType = 'fonts';
                        }

                        return `${assetType}/[name]-[hash][extname]`;
                    },
                    chunkFileNames: 'js/[name]-[hash].js',
                    entryFileNames: 'js/[name]-[hash].js',

                },
                treeshake: false
            },
        },
        resolve: {
            alias: {
                '@': SRC_DIR,
            },
        },
        server: {
            host: '0.0.0.0',
            port: 8081,
            strictPort: true,
            proxy: {
                '/server_settings.js': {
                    target: process.env['CYLEDGER_DEV_BACKEND'] || 'http://127.0.0.1:8080/',
                    changeOrigin: true
                },
                '/mobile/server_settings.js': {
                    target: process.env['CYLEDGER_DEV_BACKEND'] || 'http://127.0.0.1:8080/',
                    changeOrigin: true
                },
                '/oauth2': {
                    target: process.env['CYLEDGER_DEV_BACKEND'] || 'http://127.0.0.1:8080/',
                    changeOrigin: true
                },
                '/api': {
                    target: process.env['CYLEDGER_DEV_BACKEND'] || 'http://127.0.0.1:8080/',
                    changeOrigin: true
                },
                '/mcp': {
                    target: process.env['CYLEDGER_DEV_BACKEND'] || 'http://127.0.0.1:8080/',
                    changeOrigin: true
                },
                '/avatar': {
                    target: process.env['CYLEDGER_DEV_BACKEND'] || 'http://127.0.0.1:8080/',
                    changeOrigin: true
                },
                '/pictures': {
                    target: process.env['CYLEDGER_DEV_BACKEND'] || 'http://127.0.0.1:8080/',
                    changeOrigin: true
                },
                '/icons': {
                    target: process.env['CYLEDGER_DEV_BACKEND'] || 'http://127.0.0.1:8080/',
                    changeOrigin: true
                },
                '/qrcode': {
                    target: process.env['CYLEDGER_DEV_BACKEND'] || 'http://127.0.0.1:8080/',
                    changeOrigin: true
                },
                '/proxy': {
                    target: process.env['CYLEDGER_DEV_BACKEND'] || 'http://127.0.0.1:8080/',
                    changeOrigin: true
                },
                '/_AMapService': {
                    target: process.env['CYLEDGER_DEV_BACKEND'] || 'http://127.0.0.1:8080/',
                    changeOrigin: true
                }
            }
        },
    };

    return options;
})
