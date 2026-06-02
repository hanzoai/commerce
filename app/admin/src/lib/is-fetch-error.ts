import { FetchError } from "@hanzo/commerce-sdk"

export const isFetchError = (error: any): error is FetchError => {
  return error instanceof FetchError
}
