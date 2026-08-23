import { Collapsible, CollapsibleContent } from "@hanzo/ui"
import { Badge, Button } from "@hanzo/commerce-ui"
import { useEffect } from "react"

import useToggleState from "@lib/hooks/use-toggle-state"
import { useFormStatus } from "react-dom"

type AccountInfoProps = {
  label: string
  currentInfo: string | React.ReactNode
  isSuccess?: boolean
  isError?: boolean
  errorMessage?: string
  clearState: () => void
  children?: React.ReactNode
  'data-testid'?: string
}

const AccountInfo = ({
  label,
  currentInfo,
  isSuccess,
  isError,
  clearState,
  errorMessage = "An error occurred, please try again",
  children,
  'data-testid': dataTestid
}: AccountInfoProps) => {
  const { state, close, toggle } = useToggleState()

  const { pending } = useFormStatus()

  const handleToggle = () => {
    clearState()
    setTimeout(() => toggle(), 100)
  }

  useEffect(() => {
    if (isSuccess) {
      close()
    }
  }, [isSuccess, close])

  return (
    <div className="text-small-regular" data-testid={dataTestid}>
      <div className="flex items-end justify-between">
        <div className="flex flex-col">
          <span className="uppercase text-ui-fg-base">{label}</span>
          <div className="flex items-center flex-1 basis-0 justify-end gap-x-4">
            {typeof currentInfo === "string" ? (
              <span className="font-semibold" data-testid="current-info">{currentInfo}</span>
            ) : (
              currentInfo
            )}
          </div>
        </div>
        <div>
          <Button
            variant="secondary"
            className="w-[100px] min-h-[25px] py-1"
            onClick={handleToggle}
            type={state ? "reset" : "button"}
            data-testid="edit-button"
            data-active={state}
          >
            {state ? "Cancel" : "Edit"}
          </Button>
        </div>
      </div>

      {/* Three regions that reveal. They were `Disclosure.Panel static` — which
          renders unconditionally and never reads the Disclosure's state — with
          the open/closed look faked by animating max-height between 0 and an
          arbitrary 1000px. So the region was always in the tree and always in
          the accessibility tree, announcing a success message that was visually
          collapsed. Collapsible takes the state it already had as a prop. */}
      <Collapsible open={!!isSuccess}>
        <CollapsibleContent data-testid="success-message">
          <Badge className="p-2 my-4" color="green">
            <span>{label} updated succesfully</span>
          </Badge>
        </CollapsibleContent>
      </Collapsible>

      <Collapsible open={!!isError}>
        <CollapsibleContent data-testid="error-message">
          <Badge className="p-2 my-4" color="red">
            <span>{errorMessage}</span>
          </Badge>
        </CollapsibleContent>
      </Collapsible>

      <Collapsible open={state}>
        <CollapsibleContent>
          <div className="grid gap-y-2 py-4">
            <div>{children}</div>
            <div className="flex items-center justify-end mt-2">
              <Button
                isLoading={pending}
                className="w-full small:max-w-[140px]"
                type="submit"
                data-testid="save-button"
              >
                Save changes
              </Button>
            </div>
          </div>
        </CollapsibleContent>
      </Collapsible>
    </div>
  )
}

export default AccountInfo
