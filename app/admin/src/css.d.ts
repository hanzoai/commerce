// Side-effect CSS imports carry no types. TypeScript 7 (tsgo) refuses an
// untyped side-effect import (TS2882); this states the obvious once so the
// native typechecker and tsc agree.
declare module '*.css'
