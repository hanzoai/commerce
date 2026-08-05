import type { AwilixContainer, ResolveOptions } from "awilix"

/**
 * The following interface acts as a bucket that other modules or the
 * utils package can fill using declaration merging
 */
export interface ModuleImplementations {}

/**
 * The Hanzo Commerce Container extends [Awilix](https://github.com/jeffijoe/awilix) to
 * provide dependency injection functionalities.
 */
export type CommerceContainer<Cradle extends object = ModuleImplementations> =
  Omit<AwilixContainer, "resolve"> & {
    resolve<K extends keyof Cradle>(
      key: K,
      resolveOptions?: ResolveOptions
    ): Cradle[K]
    resolve<T>(key: string, resolveOptions?: ResolveOptions): T

    /**
     * @ignore
     */
    registerAdd: <T>(name: string, registration: T) => CommerceContainer
    /**
     * @ignore
     */
    createScope: () => CommerceContainer
  }

export type ContainerLike = {
  resolve<T = unknown>(key: string): T
}
