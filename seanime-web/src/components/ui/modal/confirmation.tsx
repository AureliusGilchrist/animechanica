"use client"

import * as React from "react"
import { Modal } from "./modal"
import { Button } from "../button/button"

export type ConfirmationOptions = {
  title?: React.ReactNode
  description?: React.ReactNode
  confirmText?: string
  cancelText?: string
  intent?: "primary" | "warning" | "success" | "alert" | "gray"
  hideCancel?: boolean
  icon?: React.ReactNode
}

export type ConfirmationDialogProps = ConfirmationOptions & {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm?: () => void | Promise<void>
  onCancel?: () => void
  loading?: boolean
}

export function ConfirmationDialog(props: ConfirmationDialogProps) {
  const {
    open,
    onOpenChange,
    onConfirm,
    onCancel,
    title = "Are you sure?",
    description,
    confirmText = "Confirm",
    cancelText = "Cancel",
    intent = "alert",
    hideCancel,
    loading,
    icon,
  } = props

  const [submitting, setSubmitting] = React.useState(false)

  const handleConfirm = async () => {
    if (submitting) return
    try {
      setSubmitting(true)
      await onConfirm?.()
      onOpenChange(false)
    } finally {
      setSubmitting(false)
    }
  }

  const handleCancel = () => {
    if (submitting) return
    onCancel?.()
    onOpenChange(false)
  }

  return (
    <Modal
      open={open}
      onOpenChange={onOpenChange}
      title={title}
      description={description}
      footer={
        <div className="flex items-center gap-2">
          {!hideCancel && (
            <Button intent="gray" onClick={handleCancel} disabled={submitting}>
              {cancelText}
            </Button>
          )}
          <Button intent={intent === "alert" ? "alert" : intent} onClick={handleConfirm} loading={loading || submitting}>
            {confirmText}
          </Button>
        </div>
      }
    >
      {/* Optional icon area */}
      {icon && (
        <div className="flex items-center mb-2">
          <span className="text-2xl mr-2">{icon}</span>
        </div>
      )}
      {/* Body can be extended by children if needed */}
    </Modal>
  )
}

// Hook for programmatic confirmations
export function useConfirmation() {
  const [open, setOpen] = React.useState(false)
  const resolverRef = React.useRef<(v: boolean) => void>()
  const [options, setOptions] = React.useState<ConfirmationOptions>({})
  const [busy, setBusy] = React.useState(false)

  const confirm = React.useCallback((opts?: ConfirmationOptions) => {
    setOptions(opts ?? {})
    setOpen(true)
    return new Promise<boolean>((resolve) => {
      resolverRef.current = resolve
    })
  }, [])

  const handleConfirm = async () => {
    setBusy(true)
    try {
      resolverRef.current?.(true)
    } finally {
      setBusy(false)
      setOpen(false)
    }
  }

  const handleCancel = () => {
    resolverRef.current?.(false)
    setOpen(false)
  }

  const dialog = (
    <ConfirmationDialog
      open={open}
      onOpenChange={setOpen}
      onConfirm={handleConfirm}
      onCancel={handleCancel}
      loading={busy}
      {...options}
    />
  )

  return { confirm, dialog }
}
