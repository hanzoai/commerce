import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
  XStack,
  YStack,
} from "@hanzo/ui"
import React from "react"

import { ModalProvider, useModal } from "@lib/context/modal-context"
import X from "@modules/common/icons/x"

/**
 * The storefront modal, on the kit's Dialog.
 *
 * The overlay, the enter/leave transition, the focus trap, the escape key and
 * the close button were all spelled out here as Headless UI `Transition.Child`
 * pairs and hand-written `opacity-0 scale-95` class strings. Every one of them
 * is `DialogContent`'s. What is left is the only thing this file ever decided:
 * how wide the panel is, and that a search modal sits at the top with no chrome.
 *
 * The public shape is unchanged — `isOpen`/`close`/`size`/`search` plus
 * `Modal.Title/Description/Body/Footer` — so no call site moves.
 */
const WIDTH = { small: 448, medium: 576, large: 768 } as const

type ModalProps = {
  isOpen: boolean
  close: () => void
  size?: keyof typeof WIDTH
  search?: boolean
  children: React.ReactNode
  "data-testid"?: string
}

const Modal = ({
  isOpen,
  close,
  size = "medium",
  search = false,
  children,
  "data-testid": dataTestId,
}: ModalProps) => (
  <Dialog modal open={isOpen} onOpenChange={(open) => !open && close()}>
    <DialogContent
      data-testid={dataTestId}
      showCloseButton={false}
      gap="$4"
      p="$5"
      maxW={WIDTH[size]}
      {...(search ? { bg: "transparent", borderWidth: 0 } : {})}
    >
      <ModalProvider close={close}>{children}</ModalProvider>
    </DialogContent>
  </Dialog>
)

const Title: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { close } = useModal()

  return (
    <XStack items="center" justify="space-between">
      <DialogTitle>{children}</DialogTitle>
      <button
        type="button"
        onClick={close}
        data-testid="close-modal-button"
        aria-label="Close"
      >
        <X size={20} />
      </button>
    </XStack>
  )
}

const Description: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <DialogDescription pt="$2" pb="$4">
    {children}
  </DialogDescription>
)

const Body: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <YStack justify="center">{children}</YStack>
)

const Footer: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <XStack items="center" justify="flex-end" gap="$4">
    {children}
  </XStack>
)

Modal.Title = Title
Modal.Description = Description
Modal.Body = Body
Modal.Footer = Footer

export default Modal
