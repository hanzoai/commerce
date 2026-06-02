"use client"

import clsx from "clsx"
import { useState } from "react"
import HomepageEdges from "../Edges"

const HomepageBloom = () => {
  const [question, setQuestion] = useState("")

  const suggestions: {
    tag: string
    questions: string[]
  }[] = [
    {
      tag: "FAQ",
      questions: [
        "What is Hanzo Commerce?",
        "How do I set up my first store?",
        "How can I extend the product data model?",
        "How do I deploy Hanzo Commerce to production?",
      ],
    },
    {
      tag: "Recipes",
      questions: [
        "How do I build a marketplace?",
        "How do I build digital products?",
        "How do I build subscription-based purchases?",
      ],
    },
  ]

  return (
    <div className="w-full flex gap-0 items-center border-y border-gray-200 dark:border-gray-800 flex-col lg:flex-row lg:h-[480px]">
      <div
        className={clsx(
          "w-full h-full lg:w-1/2 bg-gray-50 dark:bg-gray-900 relative",
          "flex flex-col justify-between gap-8",
          "p-8 border-r border-gray-200 dark:border-gray-800",
          "border-b lg:border-b-0"
        )}
      >
        <div className="flex flex-col gap-4 flex-1">
          <span className="text-2xl font-bold">
            AI-Powered Commerce. How can we help you?
          </span>
          <div className="w-full flex-1 py-3 border-t border-gray-200 dark:border-gray-800 relative">
            <textarea
              className={clsx(
                "appearance-none text-base placeholder:text-gray-400",
                "bg-transparent resize-none w-full focus:outline-none",
                "h-6 lg:h-auto"
              )}
              placeholder="Ask anything about Hanzo Commerce..."
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
            ></textarea>
          </div>
        </div>
        <HomepageEdges />
      </div>
      <div
        className={clsx(
          "w-full h-full lg:w-1/2 flex justify-start items-center",
          "px-8 py-8 lg:py-0"
        )}
      >
        <div className="flex gap-6 flex-col">
          {suggestions.map((section, index) => (
            <div className="flex gap-3 flex-col" key={index}>
              <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">
                {section.tag}
              </span>
              <div className="flex gap-2 flex-col">
                {section.questions.map((q, qIndex) => (
                  <div className="w-fit" key={qIndex}>
                    <a
                      href={`/learn`}
                      className={clsx(
                        "flex px-2 py-1 appearance-none text-left",
                        "rounded bg-gray-100 dark:bg-gray-800 font-mono",
                        "text-xs text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                      )}
                    >
                      {q}
                    </a>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

export default HomepageBloom
