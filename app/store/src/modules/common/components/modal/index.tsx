"use client"

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@hanzo/ui"
import { clx } from "@hanzo/commerce-ui"
import React from "react"

import { ModalProvider, useModal } from "@lib/context/modal-context"
import X from "@modules/common/icons/x"

const MAX_WIDTH = {
  small: 448,
  medium: 576,
  large: 768,
} as const

type ModalProps = {
  isOpen: boolean
  close: () => void
  size?: "small" | "medium" | "large"
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
}: ModalProps) => {
  return (
    <Dialog open={isOpen} onOpenChange={(open: boolean) => !open && close()}>
      <DialogContent
        showCloseButton={false}
        data-testid={dataTestId}
        style={{
          width: "100%",
          maxWidth: MAX_WIDTH[size],
          maxHeight: "75vh",
          ...(search
            ? { background: "transparent", boxShadow: "none", borderWidth: 0 }
            : null),
        }}
        className={clx("flex flex-col justify-start p-5 text-left", {
          "items-start": search,
        })}
      >
        <ModalProvider close={close}>{children}</ModalProvider>
      </DialogContent>
    </Dialog>
  )
}

const Title: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { close } = useModal()

  return (
    <DialogTitle className="flex items-center justify-between">
      <div className="text-large-semi">{children}</div>
      <div>
        <button onClick={close} data-testid="close-modal-button">
          <X size={20} />
        </button>
      </div>
    </DialogTitle>
  )
}

const Description: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return (
    <DialogDescription className="flex text-small-regular text-ui-fg-base items-center justify-center pt-2 pb-4 h-full">
      {children}
    </DialogDescription>
  )
}

const Body: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return <div className="flex justify-center">{children}</div>
}

const Footer: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return <div className="flex items-center justify-end gap-x-4">{children}</div>
}

Modal.Title = Title
Modal.Description = Description
Modal.Body = Body
Modal.Footer = Footer

export default Modal
