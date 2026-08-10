import { describe, it, expect } from "vitest"
import { mount } from "@vue/test-utils"
import Avatar from "../Avatar.vue"

describe("Avatar", () => {
  it("renders initials when no src", () => {
    const wrapper = mount(Avatar, { props: { alt: "Alice Johnson" } })
    expect(wrapper.text()).toContain("AJ")
  })

  it("renders fallback for empty alt", () => {
    const wrapper = mount(Avatar, { props: { alt: "" } })
    expect(wrapper.text()).toContain("?")
  })

  it("renders image when src provided", () => {
    const wrapper = mount(Avatar, { props: { src: "https://cdn.example/a.png", alt: "Alice" } })
    const img = wrapper.find("img")
    expect(img.exists()).toBe(true)
    expect(img.attributes("src")).toBe("https://cdn.example/a.png")
  })

  it("applies custom size", () => {
    const wrapper = mount(Avatar, { props: { size: 64 } })
    expect(wrapper.attributes("style")).toContain("64px")
  })

  it("shows online indicator", () => {
    const wrapper = mount(Avatar, { props: { online: true } })
    expect(wrapper.find("span").exists()).toBe(true)
  })

  it("hides online indicator when offline", () => {
    const wrapper = mount(Avatar, { props: { online: false } })
    expect(wrapper.find("span").exists()).toBe(false)
  })

  it("handles single-word names", () => {
    const wrapper = mount(Avatar, { props: { alt: "Alice" } })
    expect(wrapper.text()).toContain("A")
  })

  it("truncates initials to two letters", () => {
    const wrapper = mount(Avatar, { props: { alt: "A B C D" } })
    expect(wrapper.text()).toContain("AB")
  })
})
