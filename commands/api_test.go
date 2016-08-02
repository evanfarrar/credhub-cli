package commands_test

import (
	"net/http"

	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"
	"github.com/pivotal-cf/cm-cli/config"
)

var _ = Describe("API", func() {
	Describe("Help", func() {
		It("displays help", func() {
			session := runCommand("api", "-h")

			Eventually(session).Should(Exit(1))
			Expect(session.Err).To(Say("api"))
			Expect(session.Err).To(Say("SERVER_URL"))
		})
	})

	Context("when the provided server url's scheme is https", func() {
		var (
			httpsServer       *Server
			apiHttpsServerUrl string
		)

		BeforeEach(func() {
			httpsServer = NewTLSServer()

			apiHttpsServerUrl = httpsServer.URL()

			httpsServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", "/info"),
					RespondWith(http.StatusOK, `{
					"app":{"version":"0.1.0 build DEV","name":"Pivotal Credential Manager"},
					"auth-server":{"url":"https://example.com","client":"bar"}
					}`),
				),
			)
		})

		AfterEach(func() {
			httpsServer.Close()
		})

		It("sets the target URL", func() {
			session := runCommand("api", apiHttpsServerUrl)

			Eventually(session).Should(Exit(0))

			session = runCommand("api")

			Eventually(session).Should(Exit(0))
			Eventually(session.Out).Should(Say(apiHttpsServerUrl))

			config := config.ReadConfig()

			Expect(config.AuthURL).To(Equal("https://example.com"))
		})

		It("sets the target URL using a flag", func() {
			session := runCommand("api", "-s", apiHttpsServerUrl)

			Eventually(session).Should(Exit(0))

			session = runCommand("api")

			Eventually(session).Should(Exit(0))
			Eventually(session.Out).Should(Say(apiHttpsServerUrl))
		})

		It("will prefer the command's argument URL over the flag's argument", func() {
			session := runCommand("api", apiHttpsServerUrl, "-s", "woooo.com")

			Eventually(session).Should(Exit(0))
			Eventually(session.Out).Should(Say(apiHttpsServerUrl))

			session = runCommand("api")

			Eventually(session).Should(Exit(0))
			Eventually(session.Out).Should(Say(apiHttpsServerUrl))
		})

		It("sets the target IP address to an https URL when no URL scheme is provided", func() {
			session := runCommand("api", httpsServer.Addr())

			Eventually(session).Should(Exit(0))
			Eventually(session.Out).Should(Say(httpsServer.URL()))

			session = runCommand("api")

			Eventually(session.Out).Should(Say(httpsServer.URL()))
		})

		Context("when the provided server url is not valid", func() {
			var (
				badServer *Server
			)

			BeforeEach(func() {
				// confirm we have original good server
				session := runCommand("api", apiHttpsServerUrl)

				Eventually(session).Should(Exit(0))

				badServer = NewTLSServer()
				badServer.AppendHandlers(
					CombineHandlers(
						VerifyRequest("GET", "/info"),
						RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			AfterEach(func() {
				badServer.Close()
			})

			It("retains previous target when the url is not valid", func() {
				// fail to validate on bad server
				session := runCommand("api", badServer.URL())

				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("The targeted API does not appear to be valid."))

				// previous value remains
				session = runCommand("api")

				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say(httpsServer.URL()))
			})
		})

		Context("saving configuration from server", func() {
			It("saves config", func() {
				session := runCommand("api", httpsServer.URL())
				Eventually(session).Should(Exit(0))

				config := config.ReadConfig()
				Expect(config.ApiURL).To(Equal(httpsServer.URL()))
				Expect(config.AuthURL).To(Equal("https://example.com"))
			})

			It("sets file permissions so that the configuration is readable and writeable only by the owner", func() {
				configPath := config.ConfigPath()
				os.Remove(configPath)
				session := runCommand("api", httpsServer.URL())
				Eventually(session).Should(Exit(0))

				statResult, _ := os.Stat(configPath)
				Expect(int(statResult.Mode())).To(Equal(0600))
			})
		})
	})

	Context("when the provided server url's scheme is http", func() {
		var (
			httpServer *Server
		)

		BeforeEach(func() {
			httpServer = NewServer()

			httpServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", "/info"),
					RespondWith(http.StatusOK, `{
					"app":{"version":"my-version","name":"Pivotal Credential Manager"},
					"auth-server":{"url":"https://example.com","client":"bar"}
					}`),
				),
			)
		})

		AfterEach(func() {
			httpServer.Close()
		})

		It("does not use TLS", func() {
			session := runCommand("api", httpServer.URL())
			Eventually(session).Should(Exit(0))
			Eventually(session.Out).Should(Say(httpServer.URL()))

			session = runCommand("api")

			Eventually(session).Should(Exit(0))
			Eventually(session.Out).Should(Say(httpServer.URL()))
		})

		It("prints warning text", func() {
			session := runCommand("api", httpServer.URL())
			Eventually(session).Should(Exit(0))
			Eventually(session).Should(Say("Warning: Insecure HTTP API detected. Data sent to this API could be intercepted" +
				" in transit by third parties. Secure HTTPS API endpoints are recommended."))
		})
	})
})
