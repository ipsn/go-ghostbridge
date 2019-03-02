// go-ghostbridge - React Native to Go bridge
// Copyright (c) 2019 Péter Szilágyi. All rights reserved.
package io.ipsn.ghostbridge;

import com.facebook.react.modules.network.OkHttpClientProvider;
import com.facebook.react.modules.network.OkHttpClientFactory;
import com.facebook.react.modules.network.ReactCookieJarContainer;

import java.io.IOException;
import java.io.StringBufferInputStream;
import java.security.KeyStore;
import java.security.SecureRandom;
import java.security.cert.CertificateException;
import java.security.cert.CertificateFactory;
import java.security.cert.X509Certificate;

import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManager;
import javax.net.ssl.TrustManagerFactory;
import javax.net.ssl.X509TrustManager;

import okhttp3.Interceptor;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;

// GhostBridge is a utility class to retrieve the security parameters of an already
// established bridge and inject the certificate, token and domain interceptor into
// React Native's HTTP client.
public class GhostBridge {
  // GhostBridge creates a new GhostBridge, both constructing the server side as
  // well as authenticating and authorizing the client HTTP library.
  public static void init(final long port, final String cert, final String token) throws Exception {
    final X509Certificate bridgeCert  = (X509Certificate)CertificateFactory.getInstance("X.509")
      .generateCertificate(new StringBufferInputStream(cert));

    // Wrap all the trust managers so they each trust out self-signer certificate
    TrustManagerFactory factory = TrustManagerFactory.getInstance("X509");
    factory.init((KeyStore) null);

    TrustManager[] trustManagers = factory.getTrustManagers();
    for (int i = 0; i < trustManagers.length; i++) {
      if (trustManagers[i] instanceof X509TrustManager) {
        final X509TrustManager current = (X509TrustManager)trustManagers[i];

        // TLS trust manager found, add security exception for own certificate
        trustManagers[i] = new X509TrustManager() {
          @Override
          public X509Certificate[] getAcceptedIssuers() {
            return current.getAcceptedIssuers();
          }

          @Override
          public void checkClientTrusted(X509Certificate[] x509Certificates, String s) throws CertificateException {
            current.checkClientTrusted(x509Certificates, s);
          }

          @Override
          public void checkServerTrusted(X509Certificate[] chain, String authType) throws CertificateException {
            // If the server authenticated itself with the trusted certificate, accept
            for (X509Certificate cert : chain) {
              if (cert.equals(bridgeCert)) {
                return;
              }
            }
            // Certificate chain not the self-signed one, delegate to system authority
            current.checkServerTrusted(chain, authType);
          }
        };
      }
    }
    final SSLContext sslContext = SSLContext.getInstance("TLS");
    sslContext.init(null, trustManagers, new SecureRandom());

    // Replace React Native's stock socket factory with the trusted self-signed version
    OkHttpClientProvider.setOkHttpClientFactory(new OkHttpClientFactory() {
      public OkHttpClient createNewNetworkModuleClient() {
        return new OkHttpClient.Builder()
        .cookieJar(new ReactCookieJarContainer())
        .sslSocketFactory(sslContext.getSocketFactory())
        .addInterceptor(new Interceptor() {
          @Override
          public Response intercept(Chain chain) throws IOException {
            // If the request is not addressed the bridge, execute directly
            Request original = chain.request();
            if (!original.url().host().equals("ghost-bridge")) {
              return chain.proceed(original);
            }
            // Request sent to the ghost-bridge, redirect to localhost
            Request redirected = original.newBuilder()
              .url(original.url().newBuilder()
                .host("localhost")
                .port((int)port)
                .build())
              .header("Authorization", "Bearer " + token)
              .build();
            return chain.proceed(redirected);
          }
        })
        .build();
      }
    });
  }
}
