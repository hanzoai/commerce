export const subscribeToNewsletter = async (email: string) => {
  try {
    const response = await fetch("https://api.hanzo.ai/newsletter/subscribe", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
    })

    if (!response.ok) {
      throw new Error("Subscription failed")
    }

    return { success: true }
  } catch (error) {
    return {
      success: false,
      message: "An error occurred. Please try again later.",
    }
  }
}
