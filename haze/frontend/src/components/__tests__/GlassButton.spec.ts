import { describe, it, expect } from "vitest"
import { mount } from "@vue/test-utils"
import GlassButton from "../GlassButton.vue"

describe("GlassButton", () => {
  it("renders slot content", () => {
    const wrapper = mount(GlassButton, { slots: { default: "Send" } })
    expect(wrapper.text()).toContain("Send")
  })

  it("defaults to primary variant", () => {
    const wrapper = mount(GlassButton)
    expect(wrapper.attributes("style")).toContain("linear-gradient")
  })

  it("applies secondary variant styles", () => {
    const wrapper = mount(GlassButton, { props: { variant: "secondary" } })
    expect(wrapper.attributes("style")).toContain("rgba(255, 255, 255, 0.06)")
  })

  it("emits click event", async () => {
    const wrapper = mount(GlassButton, { slots: { default: "Click" } })
    await wrapper.trigger("click")
    expect(wrapper.emitted("click")).toHaveLength(1)
  })
})
