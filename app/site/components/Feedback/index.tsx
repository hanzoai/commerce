"use client"

type FeedbackProps = {
  question?: string
  className?: string
  showDottedSeparator?: boolean
  questionClassName?: string
}

const Feedback = ({ question = "Was this chapter helpful?", className }: FeedbackProps) => {
  return (
    <div className={className}>
      <p className="text-sm text-gray-500">{question}</p>
    </div>
  )
}

export default Feedback
