function hide(event) {
  node = event.target;
  div = node.nextElementSibling;

  if (div.hasAttribute("hidden")) {
    div.removeAttribute("hidden");
  } else {
    div.setAttribute("hidden", "");
  }
}
