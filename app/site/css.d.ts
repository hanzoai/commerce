// Side-effect CSS imports carry no types. TypeScript 7 (tsc, the native
// compiler) refuses an untyped side-effect import (TS2882); this states the
// obvious once so the whole workspace typechecks the same way.
declare module '*.css'
