/**
 * CSS side-effect imports. TS7 resolves module specifiers itself and a
 * stylesheet has no type declarations; this names every *.css import as a
 * side-effect-only module, which is exactly what Next makes of them.
 */
declare module "*.css"
