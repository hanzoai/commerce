import clsx from "clsx"

type HomepageEdgesProps = {
  className?: string
}

const HomepageEdges = ({ className }: HomepageEdgesProps) => {
  return (
    <>
      <span
        className={clsx(
          "absolute -top-px -left-px w-[5px] h-[5px] border-t border-l border-hanzo-fg-subtle",
          className
        )}
      />
      <span
        className={clsx(
          "absolute -top-px -right-px w-[5px] h-[5px] border-t border-r border-hanzo-fg-subtle",
          className
        )}
      />
      <span
        className={clsx(
          "absolute -bottom-px -left-px w-[5px] h-[5px] border-b border-l border-hanzo-fg-subtle",
          className
        )}
      />
      <span
        className={clsx(
          "absolute -bottom-px -right-px w-[5px] h-[5px] border-b border-r border-hanzo-fg-subtle",
          className
        )}
      />
    </>
  )
}

export default HomepageEdges
